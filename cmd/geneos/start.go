package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func init() {
	commands["start"] = Command{commandStart, parseArgs, "start"}
}

func commandStart(ct ComponentType, args []string) (err error) {
	return loopCommand(startInstance, ct, args)
}

func startInstance(c Instance) (err error) {
	pid, err := findProc(c)
	if err == nil {
		log.Println(Type(c), Name(c), "already running with PID", pid)
		return nil
	}

	if isDisabled(c) {
		return ErrDisabled
	}

	cmd, env := buildCommand(c)
	if cmd == nil {
		return fmt.Errorf("buildCommand returned nil")
	}

	if !canControl(c) {
		// fail early
		return ErrPermission
	}

	// set underlying user for child proc
	username := getString(c, Prefix(c)+"User")
	// pass possibly empty string down to setuser - it handles defaults
	if err = setuser(cmd, username); err != nil {
		return
	}

	cmd.Env = append(os.Environ(), env...)

	errfile := filepath.Join(getString(c, Prefix(c)+"LogD"), Type(c).String()+".txt")

	out, err := os.OpenFile(errfile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// if we've set-up privs at all, set the redirection output file to the same
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Credential != nil {
		if err = out.Chown(int(cmd.SysProcAttr.Credential.Uid), int(cmd.SysProcAttr.Credential.Gid)); err != nil {
			log.Println("chown:", err)
		}
	}
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Dir = filepath.Join(Home(c))

	if err = cmd.Start(); err != nil {
		return
	}
	log.Println(Type(c), Name(c), "started with PID", cmd.Process.Pid)

	if cmd.Process != nil {
		// detach from control
		cmd.Process.Release()
	}

	return
}
