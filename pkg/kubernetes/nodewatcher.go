package kubernetes

import (
	"log"
	"sync"
	"time"
)

const (
	scanPeriod = 5 * time.Second
)

type NodeWatcher struct {
	adapter   KubeAdapter
	isRunning bool
	isClosed  bool
	done      chan struct{}
	quit      chan struct{}

	nodes  chan []*Node
	errors chan error
}

func (nw *NodeWatcher) Start() {
	nw.checkClosed()

	log.Println("NodeWatcher starts")

	if nw.isRunning {
		return
	}

	nw.isRunning = true

	timer := time.NewTicker(scanPeriod)

	for {
		select {
		case <-timer.C:
			log.Println("Checking for nodes...")

			nodes, err := nw.adapter.GetNodes()
			if err != nil {
				nw.errors <- err
				continue
			}

			nw.nodes <- nodes
		case <-nw.quit:
			timer.Stop()

			close(nw.errors)
			close(nw.nodes)
			close(nw.done)

			return
		}
	}
}

func (nw *NodeWatcher) Stop() {
	nw.checkClosed()

	if !nw.isRunning {
		return
	}

	close(nw.quit)

	<-nw.done

	nw.isClosed = true

	log.Println("NodeWatcher ends")
}

func (nw *NodeWatcher) Nodes() <-chan []*Node {
	return nw.nodes
}

func (nw *NodeWatcher) Errors() <-chan error {
	return nw.errors
}

func NewNodeWatcher(adapter KubeAdapter) *NodeWatcher {
	return &NodeWatcher{
		adapter:   adapter,
		isRunning: false,
		isClosed:  false,
		done:      make(chan struct{}, 1),
		quit:      make(chan struct{}, 1),
		nodes:     make(chan []*Node, 1),
		errors:    make(chan error, 1),
	}
}

func (nw *NodeWatcher) checkClosed() {
	if nw.isClosed {
		panic("NodeWatcher already closed")
	}
}

func StartNodeWatcher(adapter KubeAdapter, quit <-chan struct{}, wg *sync.WaitGroup) *NodeWatcher {
	wg.Add(1)

	watcher := NewNodeWatcher(adapter)
	go func() {
		defer wg.Done()

		go watcher.Start()

		<-quit

		watcher.Stop()
	}()

	return watcher
}
