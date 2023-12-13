package cmd

import (
	"bytes"
	"context"
	"fmt"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/manifoldco/promptui"
	nineinfrav1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	PrintFmtStrClusterList        = "%-20s\t%-10s\t%-10s\t%-10s\t%-10s\n"
	PrintFmtStrToolList           = "%-20s\t%-10s\t%-10s\t%-10s\t%-10s\n"
	PrintFmtStrClusterProjectList = "%-40s\t%-10s\t%-10s\t%-10s\t%-10s\n"
)

var Err2Suggestions = map[string]string{
	"connection timed out": "If you run the nine out of the k8s? or if the status of the NineCluster is not ready?",
}

func runCommand(command string, args ...string) (string, string, error) {
	cmd := exec.Command(command, args...)

	var output, errput bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &errput
	err := cmd.Run()
	if output.Len() != 0 {
		//avoid return twice
		err = nil
	}
	if DEBUG {
		fmt.Printf("Exec %s args:%v with output:%s,errput:%s,err:%v\n", command, args, output.String(), errput.String(), err)
	}
	return output.String(), errput.String(), err
}

func CheckNineClusterExist(name string, namespace string) (bool, *nineinfrav1alpha1.NineClusterList) {
	path, _ := rootCmd.Flags().GetString(kubeconfig)
	nc, err := GetNineInfraClient(path)
	if err != nil {
		return false, nil
	}

	if name != "" {
		c, err := nc.NineinfraV1alpha1().NineClusters(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return true, &nineinfrav1alpha1.NineClusterList{
				Items: []nineinfrav1alpha1.NineCluster{
					*c,
				},
			}
		}
	} else {
		clist, err := nc.NineinfraV1alpha1().NineClusters(namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil && len(clist.Items) != 0 {
			return true, clist
		}
	}
	return false, nil
}

// Ask user for Y/N input. Return true if response is "y"
func Ask(label string) bool {
	prompt := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
		Default:   "n",
	}
	_, err := prompt.Run()
	return err == nil
}

func HumanDuration(t time.Time) string {
	return duration.HumanDuration(metav1.Now().Sub(t))
}

func NineWorkLoadName(name string, project string) string {
	return name + NineClusterProjectNameSuffix[project]
}

func NineToolResourceName(suffix string) string {
	return DefaultToolsNamePrefix + suffix
}

func CheckStsIfReady(name string, namespace string) bool {
	path, _ := rootCmd.Flags().GetString(kubeconfig)
	client, err := GetKubeClient(path)
	if err != nil {
		return false
	}
	sts, err := client.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return sts.Status.ReadyReplicas == *sts.Spec.Replicas
}

func CheckDeployIfReady(name string, namespace string) bool {
	path, _ := rootCmd.Flags().GetString(kubeconfig)
	client, err := GetKubeClient(path)
	if err != nil {
		return false
	}
	deploy, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return deploy.Status.ReadyReplicas == *deploy.Spec.Replicas
}

func PrintStsReadyAndAge(name string, namespace string) (string, string) {
	path, _ := rootCmd.Flags().GetString(kubeconfig)
	client, err := GetKubeClient(path)
	if err != nil {
		return "", ""
	}
	sts, err := client.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "0/0", "0s"
		} else {
			return "", ""
		}
	}

	return fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, *sts.Spec.Replicas), fmt.Sprintf("%s", HumanDuration(sts.CreationTimestamp.Time))
}

func PrintDeployReadyAndAge(name string, namespace string) (string, string) {
	path, _ := rootCmd.Flags().GetString(kubeconfig)
	client, err := GetKubeClient(path)
	if err != nil {
		return "", ""
	}
	deploy, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "0/0", "0s"
		} else {
			return "", ""
		}
	}

	return fmt.Sprintf("%d/%d", deploy.Status.ReadyReplicas, *deploy.Spec.Replicas), fmt.Sprintf("%s", HumanDuration(deploy.CreationTimestamp.Time))
}

func IfPGReady(pg *cnpgv1.Cluster) bool {
	return pg.Status.ReadyInstances == pg.Spec.Instances
}

func CheckPGClusterIfReady(name string, namespace string) bool {
	path, _ := rootCmd.Flags().GetString(kubeconfig)
	client, err := GetPGOperatorClient(path)
	if err != nil {
		return false
	}
	pg, err := client.PostgresqlV1().Clusters(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return IfPGReady(pg)
}

func PrintPGClusterReadyAndAge(name string, namespace string) (string, string) {
	path, _ := rootCmd.Flags().GetString(kubeconfig)
	client, err := GetPGOperatorClient(path)
	if err != nil {
		return "", ""
	}
	pg, err := client.PostgresqlV1().Clusters(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "0/0", "0s"
		} else {
			return "", ""
		}
	}

	return fmt.Sprintf("%d/%d", pg.Status.ReadyInstances, pg.Spec.Instances), fmt.Sprintf("%s", HumanDuration(pg.CreationTimestamp.Time))
}

func CheckClusterIfReady(name string, namespace string) bool {
	for k, v := range NineClusterProjectWorkloadList {
		switch v {
		case "statefulset":
			if !CheckStsIfReady(NineWorkLoadName(name, k), namespace) {
				return false
			}
		case "cluster":
			if !CheckPGClusterIfReady(NineWorkLoadName(name, k), namespace) {
				return false
			}
		}
	}
	return true
}

func PrintToolAccessInfo(name string, ns string) string {
	svcName := NineToolResourceName(NineToolSvcList[name])
	ip, port := GetSvcAccessInfo(svcName, NineToolPortNameList[name], ns)
	if NineToolPortProtocolList[name] != "" {
		return fmt.Sprintf("%s://%s:%d", NineToolPortProtocolList[name], ip, port)
	} else {
		return fmt.Sprintf("%s:%d", ip, port)
	}
}

func PrintClusterProjectList(name string, namespace string) {
	for k, v := range NineClusterProjectWorkloadList {
		switch v {
		case "statefulset":
			ready, age := PrintStsReadyAndAge(NineWorkLoadName(name, k), namespace)
			if ready != "" && age != "" {
				fmt.Printf(PrintFmtStrClusterProjectList, NineWorkLoadName(name, k), k, v, ready, age)
			}
		case "cluster":
			ready, age := PrintPGClusterReadyAndAge(NineWorkLoadName(name, k), namespace)
			if ready != "" && age != "" {
				fmt.Printf(PrintFmtStrClusterProjectList, NineWorkLoadName(name, k), k, v, ready, age)
			}
		}
	}
}

func PrintClusterToolList(name string, namespace string) {
	for k, v := range NineToolList {
		if !CheckHelmReleaseExist(NineToolResourceName(k), namespace) {
			continue
		}
		var readys = 0
		var notreadys = 0
		for s, w := range v.(map[string]string) {
			switch w {
			case "statefulset":
				if CheckStsIfReady(NineToolResourceName(s), namespace) {
					readys++
				} else {
					notreadys++
				}
			case "deployment":
				if CheckDeployIfReady(NineToolResourceName(s), namespace) {
					readys++
				} else {
					notreadys++
				}
			}
		}
		if readys != 0 || notreadys != 0 {
			fmt.Printf(PrintFmtStrToolList, name, k, fmt.Sprintf("%d/%d", readys, readys+notreadys), namespace, PrintToolAccessInfo(k, namespace))
		}
	}
}

func PrintToolList(clusters *nineinfrav1alpha1.NineClusterList) {
	fmt.Printf(PrintFmtStrToolList, "NINENAME", "TOOLNAME", "READY", "NAMESPACE", "ACCESS")
	for _, cluster := range clusters.Items {
		PrintClusterToolList(cluster.Name, cluster.Namespace)
	}
}

func PrintClusterList(clusters *nineinfrav1alpha1.NineClusterList) {
	fmt.Printf(PrintFmtStrClusterList, "NAME", "DATAVOLUME", "READY", "NAMESPACE", "AGE")
	for _, cluster := range clusters.Items {
		ready := fmt.Sprintf("%t", CheckClusterIfReady(cluster.Name, cluster.Namespace))
		age := fmt.Sprintf("%s", HumanDuration(cluster.CreationTimestamp.Time))
		datavolume := fmt.Sprintf("%d", cluster.Spec.DataVolume)
		fmt.Printf(PrintFmtStrClusterList, cluster.Name, datavolume, ready, cluster.Namespace, age)
	}
}

func PrintNineCluster(cluster *nineinfrav1alpha1.NineCluster) {
	fmt.Printf(PrintFmtStrClusterProjectList, "NAME", "PROJECT", "TYPE", "READY", "AGE")
	PrintClusterProjectList(cluster.Name, cluster.Namespace)
}

func GiveSuggestionsByError(err error) string {
	if err != nil {
		for k, v := range Err2Suggestions {
			if strings.Contains(err.Error(), k) {
				return v
			}
		}
	}
	return fmt.Sprintf("I'm sorry, this error is not in my knowledge base. \n" +
		"Could you please submit an issue on GitHub to help me improve my knowledge base? Thank you!")
}

func GetSvcAccessInfo(svcName string, portName string, ns string) (string, int32) {
	path, _ := rootCmd.Flags().GetString(kubeconfig)
	client, config, err := GetKubeClientWithConfig(path)
	if err != nil {
		return "", 0
	}
	svc, err := client.CoreV1().Services(ns).Get(context.TODO(), svcName, metav1.GetOptions{})
	if err != nil {
		return "", 0
	}
	var accessIP string
	var accessPort int32
	switch svc.Spec.Type {
	case corev1.ServiceTypeClusterIP:
		accessIP = svc.Spec.ClusterIP
		for _, v := range svc.Spec.Ports {
			if v.Name == portName {
				accessPort = v.Port
			}
		}
	case corev1.ServiceTypeNodePort:
		re := regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
		accessIP = re.FindString(config.Host)
		for _, v := range svc.Spec.Ports {
			if v.Name == portName {
				accessPort = v.NodePort
			}
		}
	}
	return accessIP, accessPort
}

func GetThriftIpAndPort(name string, ns string) (string, int32) {
	svcName := name + DefaultNineSuffix + "-kyuubi"
	return GetSvcAccessInfo(svcName, DefaultThriftPortName, ns)
}
