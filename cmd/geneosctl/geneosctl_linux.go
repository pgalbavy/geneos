//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

var itrsHome string = "/opt/itrs"

func refresh(c Component) {
	pid, err := findProc(c)
	if err != nil {
		return
	}

	// send a SIGUSR1
	proc, _ := os.FindProcess(pid)
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		log.Println(Type(c), Name(c), "refresh failed")
		return
	}
}

// locate a process by compoent type and name
//
// the component type must be part of the basename of the executable and
// the component name must be on the command line as an exact and standalone
// args
//
func findProc(c Component) (int, error) {
	var pids []int

	DebugLogger.Println("looking for", Type(c), Name(c))
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
			if !errors.Is(err, fs.ErrPermission) {
				DebugLogger.Printf("reading %q failed, err: %q\n", pid, err)
			}
			// an error can be transient, just debug and ignore
			continue
		}
		args := bytes.Split(data, []byte("\000"))
		bin := filepath.Base(string(args[0]))
		if strings.HasPrefix(bin, Type(c).String()) {
			for _, arg := range args[1:] {
				if string(arg) == Name(c) {
					// log.Println(pid, "matches", bin)
					return pid, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("not found")
}
