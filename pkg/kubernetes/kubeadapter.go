package kubernetes

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"time"
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
	GetNodeInformer(addFunc func(node *v1.Node), deleteFunc func(node *v1.Node)) *NodeInformer
}

type CmdKubeAdapter struct {
	clientset *kubernetes.Clientset
}

type NodeInformer struct {
	addListener    func(node *v1.Node)
	deleteListener func(node *v1.Node)

	stop chan struct{}
}

func (ni *NodeInformer) addFunc(node interface{}) {
	n := node.(*v1.Node)
	log.Printf("A node has been added: %s", n.Name)

	ni.addListener(n)
}

func (ni *NodeInformer) deleteFunc(node interface{}) {
	n := node.(*v1.Node)
	log.Printf("A node has been removed: %s", n.Name)

	ni.deleteListener(n)
}

func (ni *NodeInformer) Stop() {
	close(ni.stop)
}

func newNodeInformer(ka *CmdKubeAdapter, addFunc func(node *v1.Node), deleteFunc func(node *v1.Node)) *NodeInformer {
	watchlist := cache.NewListWatchFromClient(ka.clientset.CoreV1().RESTClient(), "nodes", v1.NamespaceAll, fields.Everything())

	ni := NodeInformer{
		addListener:    addFunc,
		deleteListener: deleteFunc,
		stop:           make(chan struct{}),
	}

	_, controller := cache.NewInformer(
		watchlist,
		&v1.Node{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    ni.addFunc,
			DeleteFunc: ni.deleteFunc,
		})

	go controller.Run(ni.stop)

	return &ni
}

func (ka *CmdKubeAdapter) GetNodeInformer(addFunc func(node *v1.Node), deleteFunc func(node *v1.Node)) *NodeInformer {
	return newNodeInformer(ka, addFunc, deleteFunc)
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
