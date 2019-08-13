package deployment

import (
	"fmt"
	"foglute/internal/model"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"log"
	"sync"
)

type Manager struct {
	usher     *DeployAnalyzer
	clientset *kubernetes.Clientset

	applications []*model.Application

	quit chan struct{}
	done chan struct{}
}

func (manager *Manager) HasApplication(application *model.Application) bool {
	for _, app := range manager.applications {
		if application.ID == app.ID {
			return true
		}
	}
	return false
}

func (manager *Manager) AddApplication(application *model.Application) error {
	if manager.HasApplication(application) {
		err := manager.redeploy(application)
		if err != nil {
			return err
		}

		// Update the application in the list
		for i, app := range manager.applications {
			if application.ID == app.ID {
				manager.applications[i] = application
				break
			}
		}
	} else {
		// Deploy the new application
		err := manager.startApplication(application)
		if err != nil {
			return err
		}

		manager.applications = append(manager.applications, application)
	}

	return nil
}

var instance *Manager

func NewDeploymentManager(usher *DeployAnalyzer, clientset *kubernetes.Clientset, quit chan struct{}) (*Manager, error) {
	if instance == nil {
		instance = &Manager{
			usher:        usher,
			clientset:    clientset,
			applications: make([]*model.Application, 0),

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

	// TODO: Check actual status of deployed applications

	return nil
}

func (manager *Manager) startApplication(application *model.Application) error {
	// TODO
	return nil
}

func (manager *Manager) redeployAll() []error {
	log.Printf("Redeploying applications (%d) for new node configuration", len(manager.applications))

	var errs []error

	// recompute all deployments
	errors := make(chan error)
	var wg sync.WaitGroup

	for _, app := range manager.applications {
		wg.Add(1)

		go func() {
			defer wg.Done()
			err := manager.redeploy(app)

			if err != nil {
				errors <- err
			}
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

func (manager *Manager) redeploy(application *model.Application) error {
	log.Printf("Redeploying application %s...", application.Name)

	if err := manager.undeploy(application); err != nil {
		log.Printf("application %s undeploy error: %s", application.Name, err)
		return err
	}

	if err := manager.deploy(application); err != nil {
		log.Printf("application %s deploy error: %s", application.Name, err)
		return err
	}

	log.Printf("Application %s redeployed successfully", application.Name)

	return nil
}

func (manager *Manager) getNodes() ([]model.Node, error) {
	knodes, err := manager.clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return convertNodes(knodes.Items), nil
}

func (manager *Manager) deploy(app *model.Application) error {
	log.Printf("Call to deploy with app: %s (%s)\n", app.ID, app.Name)
	// TODO
	return nil
}

func (manager *Manager) undeploy(app *model.Application) error {
	log.Printf("Call to undeploy with app: %s (%s)\n", app.ID, app.Name)
	// TODO
	return nil
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
