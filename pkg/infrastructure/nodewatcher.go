package infrastructure

import (
	v1 "k8s.io/api/core/v1"
	"log"
	"time"
)

const (
	scanPeriod = 5 * time.Second
)

type NodeWatcher struct {
	adapter   *KubeAdapter
	isRunning bool
	isClosed  bool
	done      chan struct{}
	quit      chan struct{}

	nodelist []v1.Node

	nodes  chan []v1.Node
	errors chan error
}

func (nw *NodeWatcher) Start() {
	nw.checkClosed()

	log.Println("NodeWatcher starts")

	if nw.isRunning {
		return
	}

	nw.isRunning = true

	//log.Println("Fetch all nodes...")
	//var err error
	//nw.nodelist, err = nw.adapter.GetNodes()
	//if err != nil {
	//	nw.errors <- err
	//
	//	nw.teardown()
	//	return
	//}
	//
	//nw.nodes <- nw.nodelist

	informer := (*nw.adapter).GetNodeInformer(func(node *v1.Node) {
		nw.nodelist = append(nw.nodelist, *node)
		nw.nodes <- nw.nodelist
	}, func(node *v1.Node) {
		var i = -1
		for idx, n := range nw.nodelist {
			if n.GetUID() == node.GetUID() {
				i = idx
				break
			}
		}

		if i >= 0 {
			nw.nodelist[len(nw.nodelist)-1], nw.nodelist[i] = nw.nodelist[i], nw.nodelist[len(nw.nodelist)-1]
			nw.nodelist = nw.nodelist[:len(nw.nodelist)-1]
			nw.nodes <- nw.nodelist
		} else {
			log.Printf("Node not found in previous list: %s", node)
		}
	})

	<-nw.quit
	informer.Stop()

	nw.teardown()
}

func (nw *NodeWatcher) teardown() {
	close(nw.errors)
	close(nw.nodes)
	close(nw.done)
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

func (nw *NodeWatcher) Nodes() <-chan []v1.Node {
	return nw.nodes
}

func (nw *NodeWatcher) Errors() <-chan error {
	return nw.errors
}

func NewNodeWatcher(adapter *KubeAdapter) *NodeWatcher {
	return &NodeWatcher{
		adapter:   adapter,
		isRunning: false,
		isClosed:  false,
		done:      make(chan struct{}, 1),
		quit:      make(chan struct{}, 1),
		nodes:     make(chan []v1.Node, 1),
		errors:    make(chan error, 1),
	}
}

func (nw *NodeWatcher) checkClosed() {
	if nw.isClosed {
		panic("NodeWatcher already closed")
	}
}

func StartNodeWatcher(adapter *KubeAdapter, quit <-chan struct{}) *NodeWatcher {
	watcher := NewNodeWatcher(adapter)
	go func() {
		go watcher.Start()

		<-quit

		watcher.Stop()
	}()

	return watcher
}
