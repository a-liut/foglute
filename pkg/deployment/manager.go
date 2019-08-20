/*
Fogluted
Microservice Fog Orchestration platform.

*/
package deployment

import (
	"fmt"
	"foglute/internal/model"
	"foglute/pkg/infrastructure"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"log"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultLinkLatency   = 99999
	defaultLinkBandwidth = 99999
)

// A Deploy represent an application that is managed by the Manager and is active on the cluster.
type Deploy struct {
	Application *model.Application `json:"application"`
	Placement   *model.Placement   `json:"placement"`
}

// The Manager is responsible to deploy deployments.
type Manager struct {
	// Analyzer to produce placements for deployments
	analyzer *DeployAnalyzer

	// Kubernetes Clientset
	clientset *kubernetes.Clientset

	// NodeWatcher on Kubernetes nodes
	nodeWatcher *infrastructure.NodeWatcher

	// Deployed deployments
	deployments []*Deploy

	// Stop channels
	quit chan struct{}
	done chan struct{}
}

func (manager *Manager) GetDeployments() []*Deploy {
	return manager.deployments
}

// Returns the application with the specified id handled by the manager.
func (manager *Manager) GetDeployByApplicationID(id string) (*Deploy, bool) {
	for _, dep := range manager.deployments {
		if dep.Application.ID == id {
			return dep, true
		}
	}

	return nil, false
}

// Returns true if the provided application is currently deployed by the manager
func (manager *Manager) HasApplication(application *model.Application) bool {
	for _, dep := range manager.deployments {
		if application.ID == dep.Application.ID {
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
		placement, err := manager.redeploy(application)
		if err != nil {
			return err
		}

		// Update the application in the list
		for i, dep := range manager.deployments {
			if application.ID == dep.Application.ID {
				manager.deployments[i].Application = application
				manager.deployments[i].Placement = placement
				break
			}
		}
	} else {
		// Deploy the new application
		placement, err := manager.deploy(application)
		if err != nil {
			return err
		}

		d := &Deploy{
			Application: application,
			Placement:   placement,
		}

		log.Printf("Adding %s to manager's active deployments", application.ID)
		manager.deployments = append(manager.deployments, d)
	}

	return nil
}

// Deletes an application from the manager.
// If the application is deployed, then it undeploy the application from the cluster
func (manager *Manager) DeleteApplication(application *model.Application) error {
	if !manager.HasApplication(application) {
		return fmt.Errorf("cannot find application %s", application.Name)
	}

	err := manager.undeploy(application)
	if err != nil {
		return err
	}

	// Remove app from the deployments list
	for i, dep := range manager.deployments {
		if dep.Application.ID == application.ID {
			manager.deployments = append(manager.deployments[:i], manager.deployments[i+1:]...)
		}
	}

	return nil
}

// Singleton pattern
var instance *Manager

// Get an instance of Manager
func NewDeploymentManager(usher *DeployAnalyzer, clientset *kubernetes.Clientset, quit chan struct{}) (*Manager, error) {
	if instance == nil {
		instance = &Manager{
			analyzer:    usher,
			clientset:   clientset,
			deployments: make([]*Deploy, 0),
			nodeWatcher: nil,

			quit: quit,
			done: make(chan struct{}),
		}

		if err := instance.init(); err != nil {
			return nil, err
		}
	}

	return instance, nil
}

// Initialize the Manager.
// It reads the current state of the Kubernetes cluster to get the actually deployed deployments.
func (manager *Manager) init() error {
	log.Printf("Initializing Assignment manager")
	// TODO: Check actual status of deployed deployments

	// Start node watcher
	w, err := infrastructure.NewNodeWatcher(manager.clientset)
	if err != nil {
		return err
	}

	instance.nodeWatcher = w

	return nil
}

// Perform the redeploy of all deployments managed by the Manager
func (manager *Manager) redeployAll() []error {
	log.Printf("Redeploying deployments (%d) for new node configuration", len(manager.deployments))

	var errs []error

	// recompute all deployments
	errors := make(chan error)
	var wg sync.WaitGroup

	for _, dep := range manager.deployments {
		wg.Add(1)

		go func() {
			defer wg.Done()
			placement, err := manager.redeploy(dep.Application)

			// Update application's placement
			dep.Placement = placement

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
func (manager *Manager) deploy(application *model.Application) (*model.Placement, error) {
	log.Printf("Call to deploy with app: %s (%s)\n", application.ID, application.Name)

	currentInfrastructure, err := manager.getInfrastructure()
	if err != nil {
		return nil, err
	}

	log.Printf("current Infrastructure: (%d) %s\n", len(currentInfrastructure.Nodes), currentInfrastructure)

	log.Printf("Getting a deployment for app %s (%s)\n", application.Name, application.ID)

	placements, err := (*manager.analyzer).GetDeployment(Normal, application, currentInfrastructure)
	if err != nil {
		return nil, err
	}

	log.Printf("Possible placements: %s\n", placements)

	best, err := pickBestPlacement(placements)
	if err != nil {
		return nil, fmt.Errorf("cannot devise a placement for app %s: %s", application.ID, err)
	}

	log.Printf("Best deployment: %s\n", best)

	err = manager.performPlacement(application, best)
	if err != nil {
		return nil, err
	}

	log.Printf("Application %s successfully deployed\n", application.ID)

	return best, nil
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
		if s.Id == assignment.ServiceID {
			service = &s
			break
		}
	}

	if service == nil {
		return nil, fmt.Errorf("service %s not found in application %s", assignment.ServiceID, application.ID)
	}

	// Image pull policy
	pullPolicy := apiv1.PullAlways
	if service.Image.Local {
		pullPolicy = apiv1.PullNever
	}

	var ports []apiv1.ContainerPort
	if len(service.Image.Ports) > 0 {
		ports = make([]apiv1.ContainerPort, len(service.Image.Ports))

		for i, port := range service.Image.Ports {
			ports[i].Name = "http"
			ports[i].Protocol = apiv1.ProtocolTCP
			ports[i].ContainerPort = int32(port.ContainerPort)
			ports[i].HostPort = int32(port.HostPort)
		}
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", application.ID, assignment.ServiceID),
			Labels: map[string]string{
				"app":      application.Name, // TODO: Use a unique ID
				"service":  assignment.ServiceID,
				"fogluted": "fogluted",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app":      application.Name, // TODO: Use a unique ID
				"service":  assignment.ServiceID,
				"fogluted": "fogluted",
			}},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":      application.Name, // TODO: Use a unique ID
						"service":  assignment.ServiceID,
						"fogluted": "fogluted",
					},
				},
				Spec: apiv1.PodSpec{
					NodeName: assignment.NodeID, // Deploy the pod to the right node only
					Containers: []apiv1.Container{
						{
							Name:            service.Id,
							Image:           service.Image.Name,
							ImagePullPolicy: pullPolicy,
							Ports:           ports,
						},
					}}},
		},
	}

	return deployment, nil
}

// Deletes an application from the Kubernetes cluster
func (manager *Manager) undeploy(application *model.Application) error {
	log.Printf("Call to undeploy with app: %s (%s)\n", application.ID, application.Name)

	deploymentsClient := manager.clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	for _, s := range application.Services {
		log.Printf("Undeploying service %s", s.Id)

		deletePolicy := metav1.DeletePropagationForeground
		if err := deploymentsClient.Delete(fmt.Sprintf("%s-%s", application.ID, s.Id), &metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			// TODO: implement a rollback procedure!
			return err
		}

		log.Printf("Deleted deployment %q.\n", s.Id)
	}

	return nil
}

// Performs the redeploy of an application
// It first undeploy the application and then deploy it again.
func (manager *Manager) redeploy(application *model.Application) (*model.Placement, error) {
	log.Printf("Redeploying application %s...", application.Name)

	if err := manager.undeploy(application); err != nil {
		log.Printf("application %s undeploy error: %s", application.Name, err)
		return nil, err
	}

	placement, err := manager.deploy(application)
	if err != nil {
		log.Printf("application %s deploy error: %s", application.Name, err)
		return nil, err
	}

	log.Printf("Application %s redeployed successfully", application.Name)

	return placement, nil
}

// Returns the infrastructure based on Kubernetes cluster nodes
func (manager *Manager) getInfrastructure() (*model.Infrastructure, error) {
	nodes, err := manager.getNodes()
	if err != nil {
		return nil, err
	}

	linksCount := (len(nodes) * (len(nodes) - 1)) / 2
	i := &model.Infrastructure{
		Nodes: nodes,
		Links: make([]model.Link, linksCount),
	}

	// Link the nodes
	// TODO: implement proper link creation strategy
	j := 0
	for idx, src := range nodes {
		for _, dst := range nodes[idx+1:] {
			i.Links[j].Probability = 1                  // TODO
			i.Links[j].Bandwidth = defaultLinkBandwidth // TODO
			i.Links[j].Latency = defaultLinkLatency     // TODO
			i.Links[j].Src = src.ID
			i.Links[j].Dst = dst.ID

			j++
		}
	}

	return i, nil
}

// Get active Kubernetes cluster nodes
func (manager *Manager) getNodes() ([]model.Node, error) {
	nodes := manager.nodeWatcher.GetNodes()

	return convertNodes(nodes), nil
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
	n := model.Node{
		ID:      string(node.GetUID()),
		Name:    node.Name,
		Address: node.Status.Addresses[0].Address,
		Location: model.Location{
			Longitude: model.NodeDefaultLongitude,
			Latitude:  model.NodeDefaultLatitude,
		},
		Profiles: make([]model.NodeProfile, 1),
	}

	if long, err := strconv.ParseInt(node.Labels["longitude"], 10, 32); err == nil {
		n.Location.Longitude = int(long)
	}

	if lat, err := strconv.ParseInt(node.Labels["latitude"], 10, 32); err == nil {
		n.Location.Latitude = int(lat)
	}

	n.Profiles[0].Probability = 1
	if iotCaps, exists := node.Labels["iot_caps"]; exists {
		n.Profiles[0].IoTCaps = strings.Split(iotCaps, ",")
	} else {
		n.Profiles[0].IoTCaps = make([]string, 0)
	}
	if secCaps, exists := node.Labels["sec_caps"]; exists {
		n.Profiles[0].SecCaps = strings.Split(secCaps, ",")
	} else {
		n.Profiles[0].SecCaps = make([]string, 0)
	}

	if hwCaps, err := strconv.ParseInt(node.Labels["hw_caps"], 10, 32); err == nil {
		n.Profiles[0].HWCaps = int(hwCaps)
	} else {
		// Default value
		n.Profiles[0].HWCaps = model.NodeDefaultHwCaps
	}

	return n
}
