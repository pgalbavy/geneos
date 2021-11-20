//go:build linux
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var itrsHome string = "/opt/itrs"

func refresh(c Component, ct ComponentType, name string) {
	pid, _, err := getPid(ct, c.dir(), name)
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

func getPid(ct ComponentType, basedir string, name string) (pid int, pidFile string, err error) {
	wd := filepath.Join(basedir, ct.String()+"s", name)
	// open pid file
	pidFile = filepath.Join(wd, ct.String()+".pid")
	pidBytes, err := ioutil.ReadFile(pidFile)
	if err != nil {
		err = fmt.Errorf("cannot read PID file")
		return
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		err = fmt.Errorf("cannot convert PID to int: %s", err)
		return
	}
	return
}
