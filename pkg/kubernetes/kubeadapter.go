package kubernetes

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
)

func Init() (*KubeAdapter, error) {
	log.Println("Checking kubernetes...")

	// creates the in-cluster config
	clientset, err := getClient("")
	if err != nil {
		panic(err.Error())
	}

	var adapter KubeAdapter
	adapter = NewCmdKubeAdapter(clientset)

	return &adapter, nil
}

func getClient(path string) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if path == "" {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", path)
	}
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

type KubeAdapter interface {
	GetNodes() ([]v1.Node, error)
}

type CmdKubeAdapter struct {
	clientset *kubernetes.Clientset
}

func (ka *CmdKubeAdapter) GetNodes() ([]v1.Node, error) {
	knodes, err := ka.clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	return knodes.Items, nil
}

func NewCmdKubeAdapter(clientset *kubernetes.Clientset) *CmdKubeAdapter {
	return &CmdKubeAdapter{clientset}
}
