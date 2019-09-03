package infrastructure

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"log"
	"sync"
	"time"
)

// A NodeWatcher listen for changes of the infrastructure - the nodes of the Kubernetes cluster - and stores them
// to let the application get the infrastructure faster.
type NodeWatcher struct {
	clientset *kubernetes.Clientset

	// Mutex on node list
	nodelistMutex *sync.Mutex

	// List of nodes
	nodelist []apiv1.Node

	// Stop channel
	stop chan struct{}
}

// Handles the addition of a node
func (nw *NodeWatcher) addFunc(node interface{}) {
	n := node.(*apiv1.Node)
	log.Printf("A node has been added: %s\n", n.Name)

	// TODO: Check the status of the node! Try to turn on node-1 after master is ready, to see if this callback is triggered again!

	// check if it is schedulable
	for _, t := range n.Spec.Taints {
		if t.Effect == apiv1.TaintEffectNoSchedule {
			// Skip no schelude node
			log.Printf("Cannot use %s for scheduling tasks\n", n.Name)
			return
		}
	}

	nw.nodelistMutex.Lock()
	nw.nodelist = append(nw.nodelist, *n)
	nw.nodelistMutex.Unlock()
}

// Handles the deletion of a node
func (nw *NodeWatcher) deleteFunc(node interface{}) {
	removedNode := node.(*apiv1.Node)
	log.Printf("A node has been removed: %s\n", removedNode.Name)

	nw.nodelistMutex.Lock()
	defer nw.nodelistMutex.Unlock()
	for i, n := range nw.nodelist {
		if n.UID == removedNode.UID {
			// Remove the node from the list
			nw.nodelist = append(nw.nodelist[:i], nw.nodelist[i+1:]...)
			return
		}
	}

	log.Println("Warning: removed node not found in previous list")
}

// Fetch all nodes from the cluster
func (nw *NodeWatcher) fetchNodes() ([]apiv1.Node, error) {
	list, err := nw.clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

// Starts the node watcher
func (nw *NodeWatcher) startWatching() {
	watchlist := cache.NewListWatchFromClient(nw.clientset.CoreV1().RESTClient(), "nodes", apiv1.NamespaceAll, fields.Everything())

	_, controller := cache.NewInformer(
		watchlist,
		&apiv1.Node{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    nw.addFunc,
			DeleteFunc: nw.deleteFunc,
		})

	go controller.Run(nw.stop)
}

// Stops the watcher
func (nw *NodeWatcher) Stop() {
	close(nw.stop)
}

// Returns the list of nodes
func (nw *NodeWatcher) GetNodes() []apiv1.Node {
	nw.nodelistMutex.Lock()
	defer nw.nodelistMutex.Unlock()

	return nw.nodelist
}

func NewNodeWatcher(clientset *kubernetes.Clientset) (*NodeWatcher, error) {
	nw := &NodeWatcher{
		clientset:     clientset,
		nodelistMutex: &sync.Mutex{},
		nodelist:      make([]apiv1.Node, 0),
		stop:          make(chan struct{}),
	}

	nw.startWatching()

	return nw, nil
}
