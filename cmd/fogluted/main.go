package main

import (
	"bytes"
	"foglute/pkg/kubernetes"
	"foglute/pkg/uds"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

	var wg sync.WaitGroup

	// Start services
	go initNodeWatcher(adapter, quit, &wg)
	go initUDSInterface(quit, &wg)

	<-stopChan

	log.Println("Stopping...")

	close(quit)
	wg.Wait()

	log.Println("FogLute ends")
}

func initNodeWatcher(adapter *kubernetes.KubeAdapter, quit chan struct{}, wg *sync.WaitGroup) {
	watcher := kubernetes.StartNodeWatcher(*adapter, quit, wg)

	for nodes := range watcher.Nodes() {
		log.Print("[")
		for _, n := range nodes {
			log.Print(n.Name)
		}
		log.Print("]")
	}
}

func initUDSInterface(quit chan struct{}, wg *sync.WaitGroup) {
	i := uds.Start(quit, wg)

	for d := range i.Data() {
		handleMessage(d)
	}
}

func handleMessage(buffer *bytes.Buffer) {
	log.Printf("Data Arrived!\n%s\n", buffer.String())
}
