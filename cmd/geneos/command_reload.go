package main

import (
	"os"
	"syscall"
)

func init() {
	commands["reload"] = Command{commandReload, parseArgs, "reload"}
	commands["refresh"] = Command{commandReload, parseArgs, "see reload"}
}

func commandReload(ct ComponentType, args []string) (err error) {
	return loopCommand(reload, ct, args)
}

func reload(c Instance) (err error) {
	pid, err := findProc(c)
	if err != nil {
		return
	}

	if !canControl(c) {
		return os.ErrPermission
	}

	// this may only mean anything on a gateway, so check component type

	// send a SIGUSR1
	proc, _ := os.FindProcess(pid)
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		log.Println(Type(c), Name(c), "refresh failed")

	}
	return
}
