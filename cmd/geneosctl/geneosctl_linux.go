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

func (c Components) refresh(ct ComponentType, name string) {
	pid, _, err := getPid(c, name)
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

func getPid(c Component, name string) (pid int, pidFile string, err error) {
	basedir := root(c)
	wd := filepath.Join(basedir, compType(c).String()+"s", name)
	// open pid file
	pidFile = filepath.Join(wd, compType(c).String()+".pid")
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
