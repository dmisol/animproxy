package main

import (
	"bufio"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const (
	idle2keep = 3
	noRxWD    = 100
	startWD   = 5
)

func NewPyServer() (p *PyDisp) {
	p = &PyDisp{started: make(map[string]*Pc)}
	p.server()
	p.wd()
	return
}

type PyDisp struct {
	mu      sync.Mutex
	sess    uint32
	started map[string]*Pc
	hot     []*Pc
}

func (p *PyDisp) wd() {
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			p.try2start()

			<-t.C

			now := time.Now()
			var keys []string
			p.mu.Lock()
			for k, v := range p.started {
				if v.T0.Add(startWD * time.Second).Before(now) {
					v.Stop("start timeout")
					keys = append(keys, k)
				}
			}
			for _, v := range keys {
				delete(p.started, v)
			}
			p.mu.Unlock()
		}
	}()
}

func (p *PyDisp) server() {
	defer log.Println(pySocket, "stopped")
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

			p.accept(fd)
		}
	}()
}

func (p *PyDisp) try2start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	st := len(p.started)

	if len(p.hot) < idle2keep && st < 2*(idle2keep) {
		for i := st; i < 2*idle2keep; i++ {
			go func() {
				id := uuid.NewString()
				c := NewPc(id)

				p.mu.Lock()
				defer p.mu.Unlock()
				p.started[id] = c
			}()
		}
	}

}

func (p *PyDisp) accept(fd net.Conn) {
	b := make([]byte, 2048)
	i, err := fd.Read(b)
	if err != nil {
		p.Println("read on accept failed", err)
		fd.Close()
		return
	}
	id := string(b[:i])

	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.started[id]
	if !ok {
		p.Println("unexpected", id)
		fd.Close()
		return
	}
	delete(p.started, id)

	c.Conn = fd

	p.hot = append(p.hot, c)
	p.Println("in queue", len(p.hot))
}

func (p *PyDisp) client2serve() (c *Pc) {
	p.mu.Lock()
	if len(p.hot) > 0 {
		c, p.hot = p.hot[0], p.hot[1:]
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	p.Println("no clients ready")
	p.try2start()

	time.Sleep(200 * time.Second)
	return p.client2serve()
}

func (p *PyDisp) Serve(sfu net.Conn) {
	defer p.try2start()

	go func() {
		sess := atomic.AddUint32(&p.sess, 1)

		p.Println("session started", sess)
		defer p.Println("session closed", sess)

		defer sfu.Close()

		pc := p.client2serve()
		defer pc.Conn.Close()

		wd := time.NewTicker(time.Second)
		defer wd.Stop()

		cntr := int32(0)
		go func() {
			defer pc.Conn.Close()
			defer sfu.Close()
			for {
				b := make([]byte, 2048)
				i, err := pc.Conn.Read(b)
				if err != nil {
					p.Println(sess, "py read", err)
					return
				}
				log.Println(sess, "p->s", string(b[:i]))
				// write and flush
				w := bufio.NewWriter(sfu)
				if _, err = w.WriteString(string(b[:i]) + "\n"); err != nil {
					p.Println(sess, "sfu write", err)
					return
				}
				w.Flush()
				atomic.StoreInt32(&cntr, 0)
			}
		}()
		go func() {
			defer pc.Conn.Close()
			defer sfu.Close()
			for {
				b := make([]byte, 2048)
				i, err := sfu.Read(b)
				if err != nil {
					p.Println(sess, "sfu read", err)
					return
				}
				log.Println(sess, "s->p", string(b[:i]))
				// write and flush
				w := bufio.NewWriter(pc.Conn)
				if _, err = w.WriteString(string(b[:i]) + "\n"); err != nil {
					p.Println(sess, "fd write", err)
					return
				}
				w.Flush()
				atomic.StoreInt32(&cntr, 0)
			}
		}()

		for {
			<-wd.C

			if atomic.AddInt32(&cntr, 1) >= noRxWD {
				pc.Stop("no rx timeout")
				return
			}
		}
	}()
}

func (p *PyDisp) Println(i ...interface{}) {
	log.Println("py", i)
}
