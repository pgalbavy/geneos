//go:build linux
package main

import (
	"log"
	"os"
	"syscall"
)

var itrsHome string = "/opt/itrs"

func refresh(c Components, ct ComponentType, name string) {
	pid, _, err := c.getPid(name)
	if err != nil {
		return
	}

	// send a SIGUSR1
	proc, _ := os.FindProcess(pid)
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		log.Println(ct, name, "reload config failed")
		return
	}
}
