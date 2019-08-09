package uds

import (
	"bytes"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

const (
	SockAddr         = "/tmp/fogluted.sock"
	DeadlineDuration = time.Duration(5) * time.Second
)

type UDSocketInterface struct {
	quit chan struct{}
	done chan struct{}

	data   chan *bytes.Buffer
	errors chan error
}

func (i *UDSocketInterface) Start() {
	log.Println("UDSocketInterface starting...")

	addr, err := net.ResolveUnixAddr("unix", SockAddr)
	if err != nil {
		log.Fatalf("failed to resolve: %v\n", err)
	}

	if err := os.RemoveAll(SockAddr); err != nil {
		log.Fatal(err)
	}

	l, err := net.ListenUnix("unix", addr)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer l.Close()

	var handlers sync.WaitGroup
	for {
		select {
		case <-i.quit:
			handlers.Wait()
			close(i.done)
			return
		default:
			// 5 seconds duration
			err = l.SetDeadline(time.Now().Add(DeadlineDuration))
			if err != nil {
				log.Fatal("Error while setting duration:", err)
			}
			// Accept new connections, dispatching them to fogLuteServer
			// in a goroutine.
			conn, err := l.AcceptUnix()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					log.Println("Timeout!")
					continue
				}
				log.Println("Failed to accept connection:", err.Error())
			}

			handlers.Add(1)
			go func() {
				defer handlers.Done()
				log.Printf("Client connected [%s]", conn.RemoteAddr().Network())
				var buf bytes.Buffer
				_, err := io.Copy(&buf, conn)
				if err != nil {
					log.Printf("Error while copying buffer: %s\n", err)
					return
				}
				err = conn.Close()
				if err != nil {
					log.Printf("Error while closing connection: %s\n", err)
				}

				i.data <- &buf
			}()
		}
	}
}

func (i *UDSocketInterface) Stop() {
	log.Println("Calling UDSocketInterface stop")
	close(i.quit)
	close(i.data)

	<-i.done

	log.Println("UDSocketInterface stopped")
}

func (i *UDSocketInterface) Data() <-chan *bytes.Buffer {
	return i.data
}

func (i *UDSocketInterface) Errors() <-chan error {
	return i.errors
}

func NewUDSSocketInterface() *UDSocketInterface {
	return &UDSocketInterface{
		quit:   make(chan struct{}, 1),
		done:   make(chan struct{}, 1),
		data:   make(chan *bytes.Buffer, 1),
		errors: make(chan error, 1),
	}
}

func Start(quit <-chan struct{}, wg *sync.WaitGroup) *UDSocketInterface {
	wg.Add(1)

	i := NewUDSSocketInterface()
	go func() {
		defer wg.Done()
		go i.Start()

		<-quit

		log.Printf("Stopping uds ")

		i.Stop()
	}()

	return i
}
