package kubernetes

import "fmt"

type KubeAdapter interface {
	GetNodes() ([]*Node, error)
}

type Node struct {
	Name             string
	Status           string
	Roles            []string
	Age              string
	Version          string
	InternalIP       string
	ExternalIP       string
	OSImage          string
	KernelVersion    string
	ContainerRuntime string
}

func (n *Node) String() string {
	return fmt.Sprintf("%s (%s)", n.Name, n.Status)
}

func (n *Node) isMaster() bool {
	for _, r := range n.Roles {
		if r == "master" {
			return true
		}
	}
	return false
}
