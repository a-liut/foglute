package main

import (
	"fmt"
	"github.com/a-liut/foglute/pkg/kubernetes"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

const (
	KubectlExec = "kubectl"
)

func main() {
	log.Println("Starting FogLute")

	stopChan := make(chan os.Signal, 1)
	quit := make(chan struct{}, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	adapter, err := initKubernetes()
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup

	startNodeWatcher(*adapter, quit, &wg)

	<-stopChan

	log.Println("Stopping...")

	close(quit)
	wg.Wait()

	log.Println("FogLute ends")
}

func initKubernetes() (*kubernetes.KubeAdapter, error) {
	log.Println("Checking kubernetes...")

	path, err := exec.LookPath(KubectlExec)
	if err != nil {
		return nil, fmt.Errorf("cannot find kubectl. %s", err)
	}
	log.Printf("using kubectl at %s\n", path)

	var adapter kubernetes.KubeAdapter
	adapter = kubernetes.NewCmdKubeAdapter(path)

	return &adapter, nil
}

func startNodeWatcher(adapter kubernetes.KubeAdapter, quit chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		watcher := kubernetes.NewNodeWatcher(adapter)
		watcher.Start()

		go func() {
			for nodes := range watcher.Nodes() {
				log.Println(nodes)
			}
		}()

		<-quit

		watcher.Stop()
	}()
}
