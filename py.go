package main

import (
	"bufio"
	"log"
	"net"
	"os"
	"os/exec"
	"sync/atomic"
	"time"
)

const (
	idle2keep = 2
	watchdog  = 10
)

func NewPyServer() (p *PySide) {
	p = &PySide{
		sockets: make(chan net.Conn, 20),
	}
	p.server()

	time.Sleep(time.Second)
	p.start()
	p.start()

	return
}

type PySide struct {
	hold    int32
	sockets chan net.Conn
	sess    uint32
}

func (p *PySide) server() {
	go func() {
		l, err := net.Listen("unix", pySocket)
		if err != nil {
			p.Println("listen", err)
			os.Exit(1)
		}

		for {
			fd, err := l.Accept()
			if err != nil {
				p.Println("accept", err)
				os.Exit(1)
			}

			p.sockets <- fd
			p.Println("in queue", atomic.AddInt32(&p.hold, 1))
		}
	}()
}

func (p *PySide) start() {
	go func() {
		t := time.Now()
		p.Println("starting")
		defer p.Println("stopped from", t)
		cmd := exec.Command("/opt/flexapix/flexatar/realtime.sh", pySocket)
		err := cmd.Run()
		p.Println("stopped", err)
	}()
}

func (p *PySide) Serve(sfu net.Conn) {
	remained := atomic.AddInt32(&p.hold, -1)
	if remained < idle2keep {
		p.start()
	}

	go func() {
		sess := atomic.AddUint32(&p.sess, 1)

		p.Println("session started", sess)
		defer p.Println("session closed", sess)

		defer sfu.Close()

		fd := <-p.sockets
		defer fd.Close()

		wd := time.NewTicker(time.Second)
		defer wd.Stop()

		cntr := int32(0)
		go func() {
			defer fd.Close()
			defer sfu.Close()
			for {
				var b []byte
				_, err := fd.Read(b)
				if err != nil {
					p.Println(sess, "py read", err)
					return
				}
				// write and flush
				w := bufio.NewWriter(sfu)
				if _, err = w.WriteString(string(b) + "\n"); err != nil {
					p.Println(sess, "sfu write", err)
					return
				}
				w.Flush()
				atomic.StoreInt32(&cntr, 0)
			}
		}()
		go func() {
			defer fd.Close()
			defer sfu.Close()
			for {
				var b []byte
				_, err := sfu.Read(b)
				if err != nil {
					p.Println(sess, "sfu read", err)
					return
				}
				// write and flush
				w := bufio.NewWriter(fd)
				if _, err = w.WriteString(string(b) + "\n"); err != nil {
					p.Println(sess, "fd write", err)
					return
				}
				w.Flush()
				atomic.StoreInt32(&cntr, 0)
			}
		}()

		for {
			<-wd.C

			if atomic.AddInt32(&cntr, 1) >= watchdog {
				p.Println("timeout", sess)
				return
			}
		}
	}()
}

func (p *PySide) Println(i ...interface{}) {
	log.Println("py", i)
}
