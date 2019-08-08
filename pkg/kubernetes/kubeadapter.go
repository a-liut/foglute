package kubernetes

import (
	"bufio"
	"log"
	"os/exec"
	"strings"
)

type CmdKubeAdapter struct {
	execPath string
}

func (ka *CmdKubeAdapter) GetNodes() ([]*Node, error) {
	cmd := exec.Command(ka.execPath, "get", "nodes", "-o", "wide")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	//stderr, err := cmd.StderrPipe()
	//if err != nil {
	//	return nil, err
	//}

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
