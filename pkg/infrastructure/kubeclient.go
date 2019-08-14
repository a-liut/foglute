package infrastructure

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Returns a Kubernetes Clientset.
func GetClientSet(path string) (*kubernetes.Clientset, error) {
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
