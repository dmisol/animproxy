package main

import (
	"context"
	"log"
	"net"
	"os/exec"
	"syscall"
	"time"
)

type Pc struct {
	Id string
	T0 time.Time
	net.Conn
	cmd *exec.Cmd
}

func NewPc(id string) (c *Pc) {
	c = &Pc{
		Id: id,
		T0: time.Now(),
	}
	c.run()
	return
}

func (c *Pc) run() {
	c.cmd = exec.CommandContext(context.Background(), "/bin/bash", "/opt/flexapix/flexatar/realtime.sh", pySocket, c.Id)
	c.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}
	err := c.cmd.Start()
	log.Println(c.Id, "start", err)
}

func (c *Pc) Stop(r string) {
	go func() {
		pgid, err := syscall.Getpgid(c.cmd.Process.Pid)
		if err == nil {
			syscall.Kill(-pgid, 15) // note the minus sign
		}

		c.cmd.Wait()
		log.Println(c.Id, "stop", r)
	}()
}
