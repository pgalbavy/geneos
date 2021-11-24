package main

import (
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"syscall"
	"time"
)

// generic action commands

func start(c Component, name string) {
	cmd, env := loadConfig(c, name)
	if cmd == nil {
		return
	}

	username := getStringWithPrefix(c, "User")
	if len(username) != 0 {
		u, _ := user.Current()
		if username != u.Username {
			// think about sudo support here
			log.Println("can't change user to", username)
			return
		}
	}

	run(c, name, cmd, env)
}

func stop(c Component, name string) {
	pid, err := findProc(c, name)
	if err != nil {
		//		log.Println("cannot get PID for", name)
		return
	}

	// send sigterm
	log.Println("stopping", Type(c), name, "with PID", pid)

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		log.Println(Type(c), "process not found")
		return
	}

	if err = proc.Signal(syscall.SIGTERM); err != nil {
		log.Println("sending SIGTERM failed:", err)
		return
	}

	// send a signal 0 in a loop
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

func run(c Component, name string, cmd *exec.Cmd, env []string) {
	// actually run the process
	cmd.Dir = getStringWithPrefix(c, "Home")
	cmd.Env = append(os.Environ(), env...)

	errfile := filepath.Join(getStringWithPrefix(c, "LogD"), name, Type(c).String()+".txt")

	out, err := os.OpenFile(errfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("cannot open output file")
	}
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Dir = filepath.Join(compRootDir(Type(c)), name)

	err = cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("process", cmd.Process.Pid)

	if cmd.Process != nil {
		// detach
		cmd.Process.Release()
	}
}
