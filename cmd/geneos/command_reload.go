package main

import (
	"os"
	"syscall"
)

func init() {
	commands["reload"] = Command{commandReload, "reload"}
	commands["refresh"] = Command{commandReload, "see reload"}
}

func commandReload(comp ComponentType, args []string) (err error) {
	return loopCommand(reload, comp, args)
}

func reload(c Component) (err error) {
	pid, err := findProc(c)
	if err != nil {
		return
	}

	if !canControl(c) {
		// fail early
		return os.ErrPermission
	}

	// send a SIGUSR1
	proc, _ := os.FindProcess(pid)
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		log.Println(Type(c), Name(c), "refresh failed")

	}
	return
}
