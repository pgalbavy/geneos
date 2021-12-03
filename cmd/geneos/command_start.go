package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func init() {
	commands["start"] = commandStart
}

func commandStart(comp ComponentType, args []string) (err error) {
	return loopCommand(start, comp, args)
}

func start(c Component) (err error) {
	log.Println("starting", Type(c), Name(c))
	cmd, env := buildCommand(c)
	if cmd == nil {
		return fmt.Errorf("buildCommand returned nil")
	}

	if !canControl(c) {
		// fail early
		return fmt.Errorf("cannot control process")
	}

	if superuser {
		// set underlying user for child proc
		username := getString(c, Prefix(c)+"User")
		err = setuid(cmd, username)
		if err != nil {
			return
		}
	}

	cmd.Env = append(os.Environ(), env...)

	errfile := filepath.Join(getString(c, Prefix(c)+"LogD"), Type(c).String()+".txt")

	out, err := os.OpenFile(errfile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open %q: %s\n", errfile, err)
	}

	if cmd.SysProcAttr != nil && superuser {
		err = out.Chown(int(cmd.SysProcAttr.Credential.Uid), int(cmd.SysProcAttr.Credential.Gid))
		if err != nil {
			log.Println("chown:", err)
		}
	}
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Dir = filepath.Join(Home(c))

	err = cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	DebugLog.Println("started process", cmd.Process.Pid)

	if cmd.Process != nil {
		// detach from control
		cmd.Process.Release()
	}

	return
}
