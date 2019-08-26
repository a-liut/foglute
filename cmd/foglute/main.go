/*
FogLute
Microservice Fog Orchestration platform.

*/
package main

import (
	"flag"
	"fmt"
	"foglute/pkg/deployment"
	"foglute/pkg/edgeusher"
	"foglute/pkg/infrastructure"
	"foglute/pkg/interface"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

func main() {
	log.Println("Starting foglute")

	rand.New(rand.NewSource(time.Now().UnixNano()))

	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	edgeUsherPath := flag.String("edgeusher", "", "absolute path to EdgeUsher folder")

	flag.Parse()

	if *edgeUsherPath == "" {
		fmt.Println("Missing EdgeUsher path")
		os.Exit(1)
	}

	stopChan := make(chan os.Signal, 1)
	quit := make(chan struct{}, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	clientset, err := infrastructure.GetClientSet(*kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	var analyzer deployment.DeployAnalyzer
	analyzer, err = edgeusher.NewEdgeUsher(*edgeUsherPath)
	if err != nil {
		log.Fatal(err)
	}

	manager, err := deployment.NewDeploymentManager(&analyzer, clientset, quit)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup

	// Start HTTP interface
	go func() {
		defer wg.Done()

		wg.Add(1)

		_interface.StartHTTPInterface(manager, quit)
	}()

	<-stopChan

	log.Println("Stopping...")

	close(quit)
	wg.Wait()

	log.Println("foglute ends")
}

// Returns the home directory
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
