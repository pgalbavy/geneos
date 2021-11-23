//go:build linux
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

var itrsHome string = "/opt/itrs"

func (c Components) refresh(ct ComponentType, name string) {
	pid, err := findProc(c, name)
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

/* func getPid(c Component, name string) (pid int, pidFile string, err error) {
	wd := filepath.Join(compRootDir(compType(c)), name)
	// open pid file
	pidFile = filepath.Join(wd, compType(c).String()+".pid")
	pidBytes, err := ioutil.ReadFile(pidFile)
	if err != nil {
		// err = fmt.Errorf("cannot read PID file")
		pid, err = findProc(c, name)
		log.Println("i am here", pid, err)

		// recreate PID ?
		return
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		err = fmt.Errorf("cannot convert PID to int: %s", err)
		return
	}
	return
} */

// locate a process by compoent type and name
//
// the component type must be part of the basename of the executable and
// the component name must be on the command line as an exact and standalone
// args
//
func findProc(c Component, name string) (int, error) {
	var pids []int

	log.Println("looking for", compType(c), name)
	// safe to ignore error as it can only be bad pattern
	dirs, _ := filepath.Glob("/proc/[0-9]*")

	for _, dir := range dirs {
		pid, _ := strconv.Atoi(filepath.Base(dir))
		pids = append(pids, pid)
	}

	sort.Ints(pids)

	for _, pid := range pids {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				continue
			}
			log.Printf("reading %q failed, err: %q\n", pid, err)
			return 0, err
		}
		args := bytes.Split(data, []byte("\000"))
		bin := filepath.Base(string(args[0]))
		if strings.HasPrefix(bin, compType(c).String()) {
			for _, arg := range args[1:] {
				if string(arg) == name {
					// log.Println(pid, "matches", bin)
					return pid, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("not found")
}
