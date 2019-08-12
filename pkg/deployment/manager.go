package deployment

import (
	"fmt"
	"foglute/internal/model"
	"foglute/pkg/kubernetes"
	v1 "k8s.io/api/core/v1"
	"log"
	"sync"
)

type Manager struct {
	usher       *DeployAnalyzer
	kubeadapter *kubernetes.KubeAdapter

	applications []*model.Application
	nodelist     []model.Node

	quit chan struct{}
	done chan struct{}
}

func (manager *Manager) SetNodes(nodes []model.Node) []error {
	if diffNodes(manager.nodelist, nodes) {
		manager.nodelist = nodes

		return manager.redeploy()
	} else {
		log.Printf("No differences between nodes")
	}

	return []error{}
}

func (manager *Manager) AddApplication(application *model.Application) error {
	err := (*manager.kubeadapter).StartApplication(application)
	if err != nil {
		return err
	}

	manager.applications = append(manager.applications, application)

	return nil
}

func (manager *Manager) redeploy() []error {
	log.Printf("Redeploying applications (%d) for new node configuration", len(manager.applications))

	var errs []error

	// recompute all deployments
	errors := make(chan error)
	var wg sync.WaitGroup

	for _, app := range manager.applications {
		wg.Add(1)

		go func() {
			defer wg.Done()
			log.Printf("Redeploying application %s...", app.Name)

			if err := manager.undeploy(app); err != nil {
				log.Printf("application %s undeploy error: %s", app.Name, err)
				errors <- err
				return
			}

			if err := manager.deploy(app); err != nil {
				log.Printf("application %s deploy error: %s", app.Name, err)
				errors <- err
				return
			}

			log.Printf("Application %s redeployed successfully", app.Name)
		}()
	}

	go func() {
		log.Printf("waiting for redeployment finishes...")
		wg.Wait()

		close(errors)
	}()

	for err := range errors {
		errs = append(errs, fmt.Errorf("an application cannot be redeployed: %s", err))
	}

	log.Printf("Redeployment finisher with %d errors", len(errs))

	return errs
}

var instance *Manager

func NewDeploymentManager(usher *DeployAnalyzer, kubeadapter *kubernetes.KubeAdapter, quit chan struct{}) (*Manager, error) {
	if instance == nil {
		instance := &Manager{
			usher:        usher,
			kubeadapter:  kubeadapter,
			applications: make([]*model.Application, 0),
			nodelist:     make([]model.Node, 0),

			quit: quit,
			done: make(chan struct{}),
		}

		err := instance.init()
		if err != nil {
			return nil, err
		}
	}

	return instance, nil
}

func (manager *Manager) init() error {
	log.Printf("Initializing Deployment manager")

	// TODO: Check actual status of nodes and deployed applications

	actualNodeList, err := (*manager.kubeadapter).GetNodes()
	if err != nil {
		return err
	}

	manager.nodelist = convertNodes(actualNodeList)

	watcher := kubernetes.StartNodeWatcher(manager.kubeadapter, manager.quit)

	go func() {
		for {
			select {
			case <-manager.quit:
				log.Println("Stopping Manager")
				close(manager.done)
				return
			case nodes := <-watcher.Nodes():
				log.Printf("New nodes config: [")
				for _, n := range nodes {
					log.Printf("%s, ", n.Name)
				}
				log.Println("]")

				list := convertNodes(nodes)

				manager.SetNodes(list)
			}
		}
	}()

	return nil
}

func (manager *Manager) deploy(app *model.Application) error {
	// TODO
	return nil
}

func (manager *Manager) undeploy(app *model.Application) error {
	// TODO
	return nil
}

func diffNodes(previous []model.Node, actual []model.Node) bool {
	log.Printf("prev: %v\nactual: %v\n", previous, actual)
	// TODO: check for differences
	return len(previous) != len(actual)
}

func convertNodes(nodes []v1.Node) []model.Node {
	ret := make([]model.Node, len(nodes))
	for i, n := range nodes {
		ret[i] = convertNode(n)
	}

	return ret
}

func convertNode(node v1.Node) model.Node {
	// TODO: To convert a node, we need to specify how to map EU properties within a KubeNode
	n := model.Node{
		ID:       string(node.GetUID()),
		Address:  node.Status.Addresses[0].Address,
		Location: model.Location{}, // TODO
		Profiles: make([]model.NodeProfile, 0),
	}

	np := model.NodeProfile{
		Probability: 1,
		HWCaps:      node.Status.Capacity.Pods().Size(),
		IotCaps:     []string{}, // TODO,
		SecCaps:     []string{}, // TODO
	}

	n.Profiles = append(n.Profiles, np)

	return n
}
