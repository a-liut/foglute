/*
Fogluted
Microservice Fog Orchestration platform.

*/
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"foglute/internal/model"
	"foglute/pkg/deployment"
	"foglute/pkg/edgeusher"
	"foglute/pkg/infrastructure"
	"foglute/pkg/uds"
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
	log.Println("Starting fogluted")

	rand.New(rand.NewSource(time.Now().UnixNano()))

	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	edgeUsherPath := flag.String("edgeusher", "", "absolute path to EdgeUsher folder")

	flag.Parse()

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

	// Start services
	go initUDSInterface(manager, quit, &wg)

	<-stopChan

	log.Println("Stopping...")

	close(quit)
	wg.Wait()

	log.Println("fogluted ends")
}

// Returns the home directory
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// Starts the Unix socket. The socket is used as input for new applications to deploy
func initUDSInterface(manager *deployment.Manager, quit chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)

	i := uds.NewUDSSocketInterface()
	go func() {
		defer wg.Done()
		go i.Start()

		<-quit

		log.Printf("Stopping uds ")

		i.Stop()
	}()

	log.Println("Waiting for applications")

	for d := range i.Data() {
		log.Println("A new application has been submitted!")
		handleMessage(manager, d)
	}

	log.Println("Data channel closed")
}

// Handles a new message arrived through the Unix socket
func handleMessage(manager *deployment.Manager, buffer *bytes.Buffer) {
	app, err := getApplicationFromBytes(buffer)
	if err != nil {
		log.Println("Cannot parse application from received data!")
		return
	}

	err = manager.AddApplication(app)
	if err != nil {
		log.Println("Cannot add application: ", err)
		return
	}

	log.Println("Application added successfully")
}

// Convert a byte buffer into an Application
func getApplicationFromBytes(buffer *bytes.Buffer) (*model.Application, error) {
	var app model.Application
	err := json.Unmarshal(buffer.Bytes(), &app)
	if err != nil {
		return nil, err
	}

	return &app, nil
}
