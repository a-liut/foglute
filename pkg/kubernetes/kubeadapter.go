package kubernetes

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

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

func Init(kubectlExec string) (*KubeAdapter, error) {
	log.Println("Checking kubernetes...")

	path, err := exec.LookPath(kubectlExec)
	if err != nil {
		return nil, fmt.Errorf("cannot find kubectl. %s", err)
	}
	log.Printf("using kubectl at %s\n", path)

	var adapter KubeAdapter
	adapter = NewCmdKubeAdapter(path)

	return &adapter, nil
}

type KubeAdapter interface {
	GetNodes() ([]*Node, error)
}

type CmdKubeAdapter struct {
	execPath string
}

func (ka *CmdKubeAdapter) GetNodes() ([]*Node, error) {
	cmd := exec.Command(ka.execPath, "get", "nodes", "-o", "wide")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	nodes := make([]*Node, 0)

	scanner := bufio.NewScanner(stdout)

	first := true
	go func() {
		for scanner.Scan() {
			if !first {
				line := scanner.Text()

				parts := strings.Fields(line)

				node := Node{
					Name:             parts[0],
					Status:           parts[1],
					Roles:            []string{parts[2]}, // TODO: enable support for multiple roles
					Age:              parts[3],
					Version:          parts[4],
					InternalIP:       parts[5],
					ExternalIP:       parts[6],
					OSImage:          parts[7],
					KernelVersion:    parts[8],
					ContainerRuntime: parts[9],
				}

				nodes = append(nodes, &node)
			} else {
				first = false
			}
		}
		if err := scanner.Err(); err != nil {
			log.Println(err)
		}
	}()

	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	return nodes, nil
}

func NewCmdKubeAdapter(path string) *CmdKubeAdapter {
	return &CmdKubeAdapter{execPath: path}
}
