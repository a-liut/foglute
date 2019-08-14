/*
Fogluted
Microservice Fog Orchestration platform.

*/
package deployment

import (
	"fmt"
	"foglute/internal/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"log"
	"sync"
)

// The Manager is responsible to deploy applications.
type Manager struct {
	// Analyzer to produce placements for applications
	analyzer *DeployAnalyzer

	// Kubernetes Clientset
	clientset *kubernetes.Clientset

	// Deployed applications
	applications []*model.Application

	// Stop channels
	quit chan struct{}
	done chan struct{}
}

// Returns true if the provided application is currently deployed by the manager
func (manager *Manager) HasApplication(application *model.Application) bool {
	for _, app := range manager.applications {
		if application.ID == app.ID {
			return true
		}
	}
	return false
}

// Adds an application to the manager.
// If the application is already deployed, the application is redeployed, otherwise
// it is deployed and added to the manager
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
		err := manager.deploy(application)
		if err != nil {
			return err
		}

		manager.applications = append(manager.applications, application)
	}

	return nil
}

// Singleton pattern
var instance *Manager

// Get an instance of Manager
func NewDeploymentManager(usher *DeployAnalyzer, clientset *kubernetes.Clientset, quit chan struct{}) (*Manager, error) {
	if instance == nil {
		instance = &Manager{
			analyzer:     usher,
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

// Initialize the Manager.
// It reads the current state of the Kubernetes cluster to get the actually deployed applications.
func (manager *Manager) init() error {
	log.Printf("Initializing Assignment manager")
	// TODO: Check actual status of deployed applications

	return nil
}

// Perform the redeploy of all applications managed by the Manager
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

	// Wait for goroutines to end
	go func() {
		log.Printf("waiting for redeployment finishes...")
		wg.Wait()

		close(errors)
	}()

	// Collect errors
	for err := range errors {
		errs = append(errs, fmt.Errorf("an application cannot be redeployed: %s", err))
	}

	log.Printf("Redeployment finisher with %d errors", len(errs))

	return errs
}

// Performs the deploy of an application
// It gets the current state of the Kubernetes cluster and produce a feasible placement for the application
func (manager *Manager) deploy(application *model.Application) error {
	log.Printf("Call to deploy with app: %s (%s)\n", application.ID, application.Name)

	currentInfrastructure, err := manager.getInfrastructure()
	if err != nil {
		return err
	}

	log.Printf("currentInfrastructure: %s\n", currentInfrastructure)

	log.Printf("Getting a deployment for app %s (%s)\n", application.Name, application.ID)

	placements, err := (*manager.analyzer).GetDeployment(Normal, application, currentInfrastructure)
	if err != nil {
		return err
	}

	log.Printf("Possible placements: %s\n", placements)

	best, err := pickBestPlacement(placements)
	if err != nil {
		return fmt.Errorf("cannot devise a placement for app %s: %s", application.ID, err)
	}

	log.Printf("Best deployment: %s\n", best)

	err = manager.performPlacement(application, best)
	if err != nil {
		return err
	}

	log.Printf("Application %s successfully deployed\n", application.ID)

	return nil
}

func pickBestPlacement(placements []model.Placement) (*model.Placement, error) {
	if len(placements) == 0 {
		return nil, fmt.Errorf("no feasible deployments")
	}
	// TODO: choose the best deployment
	return &placements[0], nil
}

func (manager *Manager) performPlacement(application *model.Application, placement *model.Placement) error {
	log.Println("Performing placement")
	for _, assignment := range placement.Assignments {
		deployment, err := manager.createDeploymentFromAssignment(application, &assignment)
		if err != nil {
			// TODO: implement a rollback procedure!
			return err
		}

		deploymentsClient := manager.clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

		result, err := deploymentsClient.Create(deployment)
		if err != nil {
			// TODO: implement a rollback procedure!
			return err
		}

		log.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
	}

	return nil
}

func (manager *Manager) createDeploymentFromAssignment(application *model.Application, assignment *model.Assignment) (*appsv1.Deployment, error) {
	var service *model.Service
	for _, s := range application.Services {
		if s.Name == assignment.ServiceName {
			service = &s
			break
		}
	}
	if service == nil {
		return nil, fmt.Errorf("service %s not found in application %s", assignment.ServiceName, application.ID)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: assignment.ServiceName,
			Labels: map[string]string{
				"app":      application.Name,       // TODO: Use a unique ID
				"service":  assignment.ServiceName, // TODO: Use a unique ID
				"fogluted": "fogluted",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app":      application.Name, // TODO: Use a unique ID
				"service":  assignment.ServiceName,
				"fogluted": "fogluted",
			}},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":      application.Name,       // TODO: Use a unique ID
						"service":  assignment.ServiceName, // TODO: Use a unique ID
						"fogluted": "fogluted",
					},
				},
				Spec: apiv1.PodSpec{Containers: []apiv1.Container{
					{
						Name:  service.Name,
						Image: service.Image,
						// TODO: Add other stuff
					},
				}}},
		},
	}

	return deployment, nil
}

// Deletes an application from the Kubernetes cluster
func (manager *Manager) undeploy(application *model.Application) error {
	log.Printf("Call to undeploy with app: %s (%s)\n", application.ID, application.Name)
	// TODO
	return nil
}

// Performs the redeploy of an application
// It first undeploy the application and then deploy it again.
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

// Returns the infrastructure based on Kubernetes cluster nodes
func (manager *Manager) getInfrastructure() (*model.Infrastructure, error) {
	nodes, err := manager.getNodes()
	if err != nil {
		return nil, err
	}

	i := &model.Infrastructure{
		Nodes: nodes,
		Links: []model.Link{},
	}

	return i, nil
}

// Get active Kubernetes cluster nodes
func (manager *Manager) getNodes() ([]model.Node, error) {
	knodes, err := manager.clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return convertNodes(knodes.Items), nil
}

// Converts a list of Kubernetes nodes to a list of Manager nodes
func convertNodes(nodes []apiv1.Node) []model.Node {
	ret := make([]model.Node, len(nodes))
	for i, n := range nodes {
		ret[i] = convertNode(n)
	}

	return ret
}

// Converts a Kubernetes node to a Manager node
func convertNode(node apiv1.Node) model.Node {
	// TODO: To convert a node, we need to specify how to map EU properties within a KubeNode
	n := model.Node{
		ID:      string(node.GetUID()),
		Name:    node.Name,
		Address: node.Status.Addresses[0].Address,
		Location: model.Location{
			Longitude: 500,
			Latitude:  500,
		}, // TODO
		Profiles: make([]model.NodeProfile, 0),
	}

	np := model.NodeProfile{
		Probability: 1,
		HWCaps:      5000,       // TODO
		IoTCaps:     []string{}, // TODO,
		SecCaps:     []string{}, // TODO
	}

	n.Profiles = append(n.Profiles, np)

	return n
}
