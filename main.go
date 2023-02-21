package main

import (
	"log"
	"net"
	"os"
	"sync/atomic"
)

const (
	pySocket  = "/tmp/py.sock"
	sfuSocket = "/tmp/sfu.sock"
)

func main() {
	defer log.Println(sfuSocket, "stopped")
	os.Remove(pySocket)
	os.Remove(sfuSocket)

	p := NewPyServer()

	l, err := net.Listen("unix", sfuSocket)
	if err != nil {
		log.Println("listen", err)
		os.Exit(1)
	}

	for {
		fd, err := l.Accept()
		if err != nil {
			log.Println("accept", err)
			os.Exit(1)
		}
		log.Println(sfuSocket, "accepted")
		p.Serve(fd)
		log.Println("in queue", atomic.AddInt32(&p.hold, 1))
	}
}
