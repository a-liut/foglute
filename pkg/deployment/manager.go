/*
 * FogLute
 *
 * A Microservice Fog Orchestration platform.
 *
 * API version: 1.0.0
 * Contact: andrea.liut@gmail.com
 */
package deployment

import (
	"fmt"
	"foglute/internal/model"
	"foglute/pkg/config"
	"foglute/pkg/infrastructure"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	defaultLinkLatency   = 1
	defaultLinkBandwidth = 99999
)

// A Deploy represent an application that is managed by the Manager and is active on the cluster.
type Deploy struct {
	Application *model.Application `json:"application"`
	Placement   *model.Placement   `json:"placement"`
}

// The Deployer component is responsible to store information about applications that are deployed by FogLute,
// managing their deployment and removal from the system.
type Manager struct {
	// Analyzer to produce placements for deployments
	analyzer *PlacementAnalyzer

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
// If the application is already deployed, nothing is done. Otherwise the application is started and added to the manager
func (manager *Manager) AddApplication(application *model.Application) []error {
	if !manager.HasApplication(application) {
		// Deploy the new application
		placement, err := manager.deploy(application)

		// Return if the deployment is not performed
		if err != nil && placement == nil {
			return err
		}

		d := &Deploy{
			Application: application,
			Placement:   placement,
		}

		log.Printf("Adding %s to manager's active deployments\n", application.ID)
		manager.deployments = append(manager.deployments, d)

		// return errors if there are some
		if err != nil {
			return err
		}
	}

	return nil
}

// Deletes an application from the manager.
// If the application is deployed, then it removes the application from the cluster
func (manager *Manager) DeleteApplication(application *model.Application) []error {
	if !manager.HasApplication(application) {
		return []error{fmt.Errorf("cannot find application %s", application.Name)}
	}

	err := manager.delete(application)

	// Remove app from the deployments list
	for i, dep := range manager.deployments {
		if dep.Application.ID == application.ID {
			manager.deployments = append(manager.deployments[:i], manager.deployments[i+1:]...)
		}
	}

	if err != nil {
		return err
	}

	return nil
}

// Singleton pattern
var instance *Manager

// Get an instance of Manager
func NewDeploymentManager(usher *PlacementAnalyzer, clientset *kubernetes.Clientset, quit chan struct{}) (*Manager, error) {
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

	log.Printf("current Infrastructure: (%d)\n", len(currentInfrastructure.Nodes))
	for _, n := range currentInfrastructure.Nodes {
		log.Printf("(%s) %s\n", n.ID, n.Name)
	}

	log.Printf("Getting a deployment for app %s (%s)\n", application.Name, application.ID)

	placements, err := (*manager.analyzer).GetPlacements(Normal, application, currentInfrastructure)
	if err != nil {
		return nil, []error{err}
	}

	log.Printf("Devised %d possible placements\n", len(placements))

	best, err := pickBestPlacement(placements)
	if err != nil {
		return nil, []error{fmt.Errorf("cannot devise a placement for app %s: %s", application.ID, err)}
	}

	// fixing ids
	ids := map[string]string{}
	for _, node := range currentInfrastructure.Nodes {
		ids[node.Name] = node.ID
	}

	for i := range best.Assignments {
		a := &best.Assignments[i]
		if id, exists := ids[a.NodeName]; exists {
			a.NodeID = id
		} else {
			return nil, []error{fmt.Errorf("cannot find node id for %s", a.NodeName)}
		}
	}

	log.Printf("Best placement: (P = %f)\n", best.Probability)
	for _, a := range best.Assignments {
		log.Printf("%s on (%s) %s\n", a.ServiceID, a.NodeID, a.NodeName)
	}

	deployErrors := manager.performPlacement(application, currentInfrastructure, best)

	elapsed := time.Since(startTime)
	log.Printf("Deploy took %v\n", elapsed)

	log.Printf("Application %s successfully deployed\n", application.ID)

	if len(deployErrors) > 0 {
		return best, deployErrors
	}

	return best, nil
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

func getPullPolicy(image model.Image) apiv1.PullPolicy {
	if image.Local {
		return apiv1.PullNever
	}

	return apiv1.PullAlways
}

func getSecContext(image model.Image) *apiv1.SecurityContext {
	tt := true
	secContext := &apiv1.SecurityContext{}
	// Set privileged mode
	if image.Privileged {
		secContext.Privileged = &tt
	}

	return secContext
}

func processEnv(image model.Image) []apiv1.EnvVar {
	env := make([]apiv1.EnvVar, len(image.Env))
	iEnv := 0
	for varName, varValue := range image.Env {
		env[iEnv].Name = varName
		env[iEnv].Value = varValue
		iEnv++
	}

	return env
}

func createServiceFromPort(application *model.Application, assignment *model.Assignment, serviceName string, port model.Port) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Labels: map[string]string{
				fmt.Sprintf("%s/app", config.FoglutePackageName):     application.Name, // TODO: Use a unique ID
				fmt.Sprintf("%s/service", config.FoglutePackageName): assignment.ServiceID,
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
				fmt.Sprintf("%s/app", config.FoglutePackageName):     application.Name, // TODO: Use a unique ID
				fmt.Sprintf("%s/service", config.FoglutePackageName): assignment.ServiceID,
			},
			Type: apiv1.ServiceTypeLoadBalancer,
		},
	}
}

func createDeployment(application *model.Application, assignment *model.Assignment, node *model.Node, containers []apiv1.Container) *appsv1.Deployment {
	deploymentName := fmt.Sprintf("%s-%s", application.ID, assignment.ServiceID)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			Labels: map[string]string{
				fmt.Sprintf("%s/app", config.FoglutePackageName):     application.Name, // TODO: Use a unique ID
				fmt.Sprintf("%s/service", config.FoglutePackageName): assignment.ServiceID,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				fmt.Sprintf("%s/app", config.FoglutePackageName):     application.Name, // TODO: Use a unique ID
				fmt.Sprintf("%s/service", config.FoglutePackageName): assignment.ServiceID,
			}},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						fmt.Sprintf("%s/app", config.FoglutePackageName):     application.Name, // TODO: Use a unique ID
						fmt.Sprintf("%s/service", config.FoglutePackageName): assignment.ServiceID,
					},
				},
				Spec: apiv1.PodSpec{
					NodeName:   node.Name, // Deploy the pod to the selected node only
					Hostname:   deploymentName,
					Containers: containers,
				}},
		},
	}
}

func cleanImageName(image model.Image) string {
	return strings.Replace(image.Name, ":", "", -1)
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
			break
		}
	}

	if node == nil {
		return nil, nil, fmt.Errorf("node %s not found in the infrastructure", assignment.NodeID)
	}

	log.Printf("Creating %s deployment...", assignment.ServiceID)

	services := make([]*apiv1.Service, 0)
	containers := make([]apiv1.Container, 0)

	for _, image := range service.Images {
		// Image pull policy
		pullPolicy := getPullPolicy(image)

		secContext := getSecContext(image)

		// Checking env variables
		log.Printf("Environment variables to set: %v\n", image.Env)

		env := processEnv(image)

		var ports []apiv1.ContainerPort
		if len(image.Ports) > 0 {
			ports = make([]apiv1.ContainerPort, len(image.Ports))

			for i, port := range image.Ports {
				ports[i].Name = "http"
				ports[i].Protocol = apiv1.ProtocolTCP
				ports[i].ContainerPort = int32(port.ContainerPort)
				ports[i].HostPort = int32(port.HostPort)

				if port.Expose > 0 {
					serviceName := port.Name

					// Create a service for it
					s := createServiceFromPort(application, assignment, serviceName, port)

					services = append(services, s)
				}
			}
		}

		cleanImageName := cleanImageName(image)
		containerName := fmt.Sprintf("%s-%s", service.Id, cleanImageName)

		// Add a container for each image found
		containers = append(containers, apiv1.Container{
			Name:            containerName,
			Image:           image.Name,
			ImagePullPolicy: pullPolicy,
			Ports:           ports,
			SecurityContext: secContext,
			Env:             env,
		})
	}

	deployment := createDeployment(application, assignment, node, containers)

	return deployment, services, nil
}

// Deletes an application from the Kubernetes cluster
func (manager *Manager) delete(application *model.Application) []error {
	log.Printf("Call to delete with app: %s (%s)\n", application.ID, application.Name)

	startTime := time.Now()

	deploymentsClient := manager.clientset.AppsV1().Deployments(apiv1.NamespaceDefault)
	serviceClient := manager.clientset.CoreV1().Services(apiv1.NamespaceDefault)

	errors := make([]error, 0)

	for _, s := range application.Services {
		deploymentName := fmt.Sprintf("%s-%s", application.ID, s.Id)
		deletePolicy := metav1.DeletePropagationForeground

		log.Printf("Deleting Deployment %s (%s)...\n", s.Id, deploymentName)

		err := deploymentsClient.Delete(deploymentName, &metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		})
		if err != nil {
			log.Printf("Cannot delete Deployment %s: %s\n", deploymentName, err)
			errors = append(errors, err)
		} else {
			log.Printf("Deployment %s deleted.\n", s.Id)
		}

		for _, image := range s.Images {
			for _, port := range image.Ports {
				if port.Expose > 0 {
					// Remove the associated service
					serviceName := port.Name

					log.Printf("Deleting Service %s...\n", serviceName)

					if err := serviceClient.Delete(serviceName, &metav1.DeleteOptions{
						PropagationPolicy: &deletePolicy,
					}); err != nil {
						log.Printf("Cannot delete Service %s: %s\n", serviceName, err)
						errors = append(errors, err)
					} else {
						log.Printf("Service %s deleted.\n", serviceName)
					}
				}
			}
		}

	}

	elapsed := time.Since(startTime)
	log.Printf("Remove took %v\n", elapsed)

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// Performs the redeploy of an application
// It first delete the application and then start it again.
func (manager *Manager) redeploy(application *model.Application) (*model.Placement, []error) {
	log.Printf("Redeploying application %s...\n", application.Name)

	if err := manager.delete(application); err != nil {
		log.Printf("Application %s delete error: %s\n", application.Name, err)
		return nil, err
	}

	placement, err := manager.deploy(application)
	if err != nil && placement == nil {
		log.Printf("Application %s deploy error: %s\n", application.Name, err)
		return nil, err
	}

	if err != nil {
		log.Printf("Errors during %s redeployment: %s", application.Name, err)
	} else {
		log.Printf("Application %s redeployed successfully\n", application.Name)
	}

	return placement, nil
}

// Returns the infrastructure based on Kubernetes cluster nodes
func (manager *Manager) getInfrastructure() (*model.Infrastructure, error) {
	nodes, err := manager.GetNodes()
	if err != nil {
		return nil, err
	}

	// Create the complete graph of node
	linksCount := len(nodes) * (len(nodes) - 1)
	i := &model.Infrastructure{
		Nodes: nodes,
		Links: make([]model.Link, linksCount),
	}

	// Link the nodes
	j := 0
	for _, src := range nodes {
		for _, dst := range nodes {
			if src.ID != dst.ID {
				i.Links[j].Probability = 1
				i.Links[j].Bandwidth = defaultLinkBandwidth
				i.Links[j].Latency = defaultLinkLatency
				i.Links[j].Src = src.Name
				i.Links[j].Dst = dst.Name

				j++
			}
		}
	}

	return i, nil
}

// Get active Kubernetes cluster nodes
func (manager *Manager) GetNodes() ([]model.Node, error) {
	nodes := manager.nodeWatcher.GetNodes()

	return convertNodes(nodes), nil
}
