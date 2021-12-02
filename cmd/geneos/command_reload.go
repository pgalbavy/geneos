package main

import (
	"fmt"
	"os"
	"syscall"
)

func init() {
	commands["reload"] = commandReload
	commands["refresh"] = commandReload
}

func commandReload(comp ComponentType, args []string) (err error) {
	for _, name := range args {
		for _, c := range New(comp, name) {
			err = loadConfig(c, false)
			if err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				return
			}
			reload(c)
		}
	}
	return
}

func reload(c Component) (err error) {
	pid, err := findProc(c)
	if err != nil {
		return
	}

	if !canControl(c) {
		// fail early
		return fmt.Errorf("cannot control process")
	}

	// send a SIGUSR1
	proc, _ := os.FindProcess(pid)
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		log.Println(Type(c), Name(c), "refresh failed")

	}
	return
}
