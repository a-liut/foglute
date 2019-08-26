/*
FogLute
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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
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
func (manager *Manager) AddApplication(application *model.Application) []error {
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

		log.Printf("Adding %s to manager's active deployments\n", application.ID)
		manager.deployments = append(manager.deployments, d)
	}

	return nil
}

// Deletes an application from the manager.
// If the application is deployed, then it undeploy the application from the cluster
func (manager *Manager) DeleteApplication(application *model.Application) []error {
	if !manager.HasApplication(application) {
		return []error{fmt.Errorf("cannot find application %s", application.Name)}
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
	log.Println("Initializing Assignment manager")
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
	log.Printf("Redeploying deployments (%d) for new node configuration\n", len(manager.deployments))

	startTime := time.Now()

	var errs []error

	// recompute all deployments
	errors := make(chan error)
	var wg sync.WaitGroup

	for _, dep := range manager.deployments {
		wg.Add(1)

		go func() {
			defer wg.Done()
			placement, deployErrors := manager.redeploy(dep.Application)

			// Update application's placement
			dep.Placement = placement

			if deployErrors != nil {
				for _, err := range deployErrors {
					errors <- err
				}
			}
		}()
	}

	// Wait for goroutines to end
	go func() {
		log.Println("Waiting for redeployment finishes...")
		wg.Wait()

		elapsed := time.Since(startTime)
		log.Printf("Redeploy took %v\n", elapsed)

		close(errors)
	}()

	// Collect errors
	for err := range errors {
		errs = append(errs, fmt.Errorf("an application cannot be redeployed: %s", err))
	}

	log.Printf("Redeployment finishes with %d errors\n", len(errs))

	return errs
}

// Performs the deploy of an application
// It gets the current state of the Kubernetes cluster and produce a feasible placement for the application
func (manager *Manager) deploy(application *model.Application) (*model.Placement, []error) {
	log.Printf("Call to deploy with app: %s (%s)\n", application.ID, application.Name)

	startTime := time.Now()

	currentInfrastructure, err := manager.getInfrastructure()
	if err != nil {
		return nil, []error{err}
	}

	log.Printf("current Infrastructure: (%d) %s\n", len(currentInfrastructure.Nodes), currentInfrastructure)

	log.Printf("Getting a deployment for app %s (%s)\n", application.Name, application.ID)

	placements, err := (*manager.analyzer).GetDeployment(Normal, application, currentInfrastructure)
	if err != nil {
		return nil, []error{err}
	}

	log.Printf("Devised %d possible placements\n", len(placements))

	best, err := pickBestPlacement(placements)
	if err != nil {
		return nil, []error{fmt.Errorf("cannot devise a placement for app %s: %s", application.ID, err)}
	}

	log.Printf("Best placement: %s\n", best)

	deployErrors := manager.performPlacement(application, currentInfrastructure, best)

	elapsed := time.Since(startTime)
	log.Printf("Deploy took %v\n", elapsed)

	log.Printf("Application %s successfully deployed\n", application.ID)

	if len(deployErrors) > 0 {
		return best, deployErrors
	}

	return best, nil
}

// Returns the best placement from a list of placements
func pickBestPlacement(placements []model.Placement) (*model.Placement, error) {
	if len(placements) == 0 {
		return nil, fmt.Errorf("no feasible deployments")
	}

	// Scan placements and pick the best ones
	list := make([]*model.Placement, 0)
	bestProb := 0.0
	for _, p := range placements {
		if bestProb < p.Probability {
			// Clear the list
			list = list[:0]

			list = append(list, &p)
		} else if bestProb == p.Probability {
			list = append(list, &p)
		}
	}

	// Pick a random placement
	idx := rand.Intn(len(list))

	return list[idx], nil
}

// Performs proper operations in order to apply the placement to the Kubernetes cluster
func (manager *Manager) performPlacement(application *model.Application, infrastructure *model.Infrastructure, placement *model.Placement) []error {
	log.Println("Performing placement")

	errors := make([]error, 0)

	deploymentsClient := manager.clientset.AppsV1().Deployments(apiv1.NamespaceDefault)
	servicesClient := manager.clientset.CoreV1().Services(apiv1.NamespaceDefault)

	for _, assignment := range placement.Assignments {
		deployment, services, err := manager.createDeploymentFromAssignment(application, infrastructure, &assignment)
		if err != nil {
			log.Printf("Cannot get Deployment and Services for application %s and assignment (%s, %s): %s\n", application.ID, assignment.ServiceID, assignment.NodeID, err)
			errors = append(errors, err)
			continue
		}

		_, err = deploymentsClient.Create(deployment)
		if err != nil {
			log.Printf("Cannot create a Deployment for application %s and assignment (%s, %s): %s\n", application.ID, assignment.ServiceID, assignment.NodeID, err)
			errors = append(errors, err)
			continue
		} else {
			log.Printf("Deployment %s created.\n", assignment.ServiceID)
		}

		for _, s := range services {
			serviceResult, err := servicesClient.Create(s)
			if err != nil {
				log.Printf("Cannot create a Service for app service %s: %s\n", assignment.ServiceID, err)
				errors = append(errors, err)
				continue
			} else {
				log.Printf("Service %s created. Ports: %v\n", s.Name, serviceResult.Spec.Ports)
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// Returns Kubernetes Deployments and Services for the given Application according to a given Assignment
func (manager *Manager) createDeploymentFromAssignment(application *model.Application, infrastructure *model.Infrastructure, assignment *model.Assignment) (*appsv1.Deployment, []*apiv1.Service, error) {
	var service *model.Service
	for _, s := range application.Services {
		if s.Id == assignment.ServiceID {
			service = &s
			break
		}
	}

	if service == nil {
		return nil, nil, fmt.Errorf("service %s not found in application %s", assignment.ServiceID, application.ID)
	}

	var node *model.Node
	for _, n := range infrastructure.Nodes {
		if n.ID == assignment.NodeID {
			node = &n
		}
	}

	if node == nil {
		return nil, nil, fmt.Errorf("node %s not found in the infrastructure", assignment.NodeID)
	}

	services := make([]*apiv1.Service, 0)

	// Image pull policy
	pullPolicy := apiv1.PullAlways
	if service.Image.Local {
		pullPolicy = apiv1.PullNever
	}

	tt := true
	secContext := &apiv1.SecurityContext{}
	// Set privileged mode
	if service.Image.Privileged {
		secContext.Privileged = &tt
	}

	var ports []apiv1.ContainerPort
	if len(service.Image.Ports) > 0 {
		ports = make([]apiv1.ContainerPort, len(service.Image.Ports))

		for i, port := range service.Image.Ports {
			ports[i].Name = "http"
			ports[i].Protocol = apiv1.ProtocolTCP
			ports[i].ContainerPort = int32(port.ContainerPort)
			ports[i].HostPort = int32(port.HostPort)

			if port.Expose > 0 {
				serviceName := port.Name

				// Create a service for it
				s := &apiv1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: serviceName,
						Labels: map[string]string{
							"app":     application.Name, // TODO: Use a unique ID
							"service": assignment.ServiceID,
							"foglute": "foglute",
						},
					},
					Spec: apiv1.ServiceSpec{
						Ports: []apiv1.ServicePort{
							{
								Protocol: "TCP",
								NodePort: int32(port.Expose),
								Port:     int32(port.HostPort),
								TargetPort: intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: int32(port.ContainerPort),
									StrVal: string(port.ContainerPort),
								},
							},
						},
						Selector: map[string]string{
							"app":     application.Name, // TODO: Use a unique ID
							"service": assignment.ServiceID,
							"foglute": "foglute",
						},
						Type: apiv1.ServiceTypeNodePort,
					},
				}

				services = append(services, s)
			}
		}
	}

	deploymentName := fmt.Sprintf("%s-%s", application.ID, assignment.ServiceID)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			Labels: map[string]string{
				"app":     application.Name, // TODO: Use a unique ID
				"service": assignment.ServiceID,
				"foglute": "foglute",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app":     application.Name, // TODO: Use a unique ID
				"service": assignment.ServiceID,
				"foglute": "foglute",
			}},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     application.Name, // TODO: Use a unique ID
						"service": assignment.ServiceID,
						"foglute": "foglute",
					},
				},
				Spec: apiv1.PodSpec{
					NodeName: node.Name, // Deploy the pod to the right node only
					Containers: []apiv1.Container{
						{
							Name:            service.Id,
							Image:           service.Image.Name,
							ImagePullPolicy: pullPolicy,
							Ports:           ports,
							SecurityContext: secContext,
						},
					}}},
		},
	}

	return deployment, services, nil
}

// Deletes an application from the Kubernetes cluster
func (manager *Manager) undeploy(application *model.Application) []error {
	log.Printf("Call to undeploy with app: %s (%s)\n", application.ID, application.Name)

	startTime := time.Now()

	deploymentsClient := manager.clientset.AppsV1().Deployments(apiv1.NamespaceDefault)
	serviceClient := manager.clientset.CoreV1().Services(apiv1.NamespaceDefault)

	errors := make([]error, 0)

	for _, s := range application.Services {
		log.Printf("Undeploying service %s\n", s.Id)

		deploymentName := fmt.Sprintf("%s-%s", application.ID, s.Id)
		deletePolicy := metav1.DeletePropagationForeground
		err := deploymentsClient.Delete(deploymentName, &metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		})

		if err != nil {
			log.Printf("Cannot delete Deployment %s: %s\n", deploymentName, err)
			errors = append(errors, err)
		} else {
			log.Printf("Deployment %s deleted.\n", s.Id)
		}

		for _, port := range s.Image.Ports {
			if port.Expose > 0 {
				// Remove the associated service
				serviceName := port.Name

				err := serviceClient.Delete(serviceName, &metav1.DeleteOptions{
					PropagationPolicy: &deletePolicy,
				})

				if err != nil {
					log.Printf("Cannot undeploy Service %s: %s\n", serviceName, err)
					errors = append(errors, err)
				} else {
					log.Printf("Service %s deleted.\n", serviceName)
				}
			}
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("Undeploy took %v\n", elapsed)

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// Performs the redeploy of an application
// It first undeploy the application and then deploy it again.
func (manager *Manager) redeploy(application *model.Application) (*model.Placement, []error) {
	log.Printf("Redeploying application %s...\n", application.Name)

	if err := manager.undeploy(application); err != nil {
		log.Printf("Application %s undeploy error: %s\n", application.Name, err)
		return nil, err
	}

	placement, err := manager.deploy(application)
	if err != nil {
		log.Printf("Application %s deploy error: %s\n", application.Name, err)
		return nil, err
	}

	log.Printf("Application %s redeployed successfully\n", application.Name)

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
