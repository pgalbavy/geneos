package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"syscall"
	"time"
)

// generic action commands

func start(c Component) {
	cmd, env := makeCmd(c)
	if cmd == nil {
		return
	}

	username := getString(c, Prefix(c)+"User")
	if len(username) != 0 {
		u, _ := user.Current()
		if username != u.Username {
			// think about sudo support here
			log.Println("can't change user to", username)
			return
		}
	}

	run(c, cmd, env)
}

func stop(c Component) {
	pid, err := findProc(c)
	if err != nil {
		//		log.Println("cannot get PID for", name)
		return
	}

	// send sigterm

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		log.Println("stopping", Type(c), Name(c), "process", pid, err)
		return
	}

	log.Println("stopping", Type(c), Name(c), "PID", pid)

	if err = proc.Signal(syscall.SIGTERM); err != nil {
		log.Println("sending SIGTERM failed:", err)
		return
	}

	// send a signal 0 in a loop to check if the process has terminated
	for i := 0; i < 10; i++ {
		time.Sleep(250 * time.Millisecond)
		if err = proc.Signal(syscall.Signal(0)); err != nil {
			log.Println(Type(c), "terminated")
			return
		}
	}
	// sigkill
	if err = proc.Signal(syscall.SIGKILL); err != nil {
		log.Println("sending SIGKILL failed:", err)
		return
	}
}

func run(c Component, cmd *exec.Cmd, env []string) {
	// actually run the process
	cmd.Dir = getString(c, Prefix(c)+"Home")
	cmd.Env = append(os.Environ(), env...)

	errfile := filepath.Join(getString(c, Prefix(c)+"LogD"), Type(c).String()+".txt")

	out, err := os.OpenFile(errfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("cannot open %q file: %s\n", errfile, err)
	}
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Dir = filepath.Join(Home(c))

	err = cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	DebugLogger.Println("started process", cmd.Process.Pid)

	if cmd.Process != nil {
		// detach
		cmd.Process.Release()
	}
}

func create(c Component) error {
	// create a directory and a default config file

	switch Type(c) {
	case Gateway:
		// gwconfs := getConfigs(Type(c))
		/* 		err := createGateway(c)
		   		if err != nil {
		   			return err
		   		} */
	default:
		// wildcard, create an environment (later)
		return fmt.Errorf("wildcard creation net yet supported")
	}

	err := os.MkdirAll(Home(c), 0775)
	if err != nil {
		return err
	}

	// update settings here, then write
	WriteConfig(c)
	return nil
}
