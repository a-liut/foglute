package main

import (
	"bytes"
	"encoding/json"
	"foglute/internal/model"
	"foglute/pkg/deployment"
	"foglute/pkg/edgeusher"
	"foglute/pkg/kubernetes"
	"foglute/pkg/uds"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	EdgeUsherPath = "/edgeusher"
)

func main() {
	log.Println("Starting fogluted")

	stopChan := make(chan os.Signal, 1)
	quit := make(chan struct{}, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	adapter, err := kubernetes.Init()
	if err != nil {
		log.Fatal(err)
	}

	var analyzer deployment.DeployAnalyzer
	analyzer, err = edgeusher.NewEdgeUsher(EdgeUsherPath)
	if err != nil {
		log.Fatal(err)
	}

	manager, err := deployment.NewDeploymentManager(&analyzer, adapter, quit)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup

	// Start services
	// go initNodeWatcher(&analyzer, adapter, quit, &wg)
	go initUDSInterface(manager, quit, &wg)

	<-stopChan

	log.Println("Stopping...")

	close(quit)
	wg.Wait()

	log.Println("fogluted ends")
}

func initUDSInterface(manager *deployment.Manager, quit chan struct{}, wg *sync.WaitGroup) {
	i := uds.Start(quit, wg)

	log.Println("Waiting for applications")

	for d := range i.Data() {
		log.Printf("Data Arrived!\n%s\n", d.String())
		handleMessage(manager, d)
	}

	log.Println("Data channel closed")
}

func handleMessage(manager *deployment.Manager, buffer *bytes.Buffer) {
	app, err := getApplicationFromBytes(buffer)
	if err != nil {
		log.Println("Cannot parse application from received data!")
		return
	}

	err = manager.AddApplication(app)
	if err != nil {
		log.Println("Cannot add application: ", err)
	}

	log.Println("Application added successfully")
}

func getApplicationFromBytes(buffer *bytes.Buffer) (*model.Application, error) {
	var app model.Application
	err := json.Unmarshal(buffer.Bytes(), &app)
	if err != nil {
		return nil, err
	}

	return &app, nil
}
