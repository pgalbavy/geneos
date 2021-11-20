package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type NetprobeComponent struct {
	Component
	ITRSHome  string
	NetpRoot  string   `default:"{{join .ITRSHome \"netprobe\"}}"`
	NetpBins  string   `default:"{{join .ITRSHome \"packages\" \"netprobe\"}}"`
	NetpBase  string   `default:"active_prod"`
	NetpLogD  string   `default:"{{join .NetpRoot \"netprobes\"}}"`
	NetpLogF  string   `default:"netprobe.log"`
	NetpMode  string   `default:"background"`
	NetpOpts  []string // =-nopassword
	NetpLibs  string   `default:"{{join .NetpBins .NetpBase \"lib64\"}}:{{join .NetpBins .NetpBase}}"`
	NetpUser  string
	BinSuffix string `default:"netprobe.linux_64"`
}

func (c NetprobeComponent) all() []string {
	probesDir := filepath.Join(c.NetpRoot, "netprobes")
	return dirs(probesDir)
}

func (c NetprobeComponent) setup(name string) (cmd *exec.Cmd, env []string) {
	wd := filepath.Join(c.NetpRoot, "netprobes", name)
	if err := os.Chdir(wd); err != nil {
		log.Println("cannot chdir() to", wd)
		return
	}
	rcFile, err := os.Open("netprobe.rc")
	if err != nil {
		log.Println("cannot open netprobe.rc")
		return
	}
	defer rcFile.Close()

	confs := make(map[string]string)
	scanner := bufio.NewScanner(rcFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		s := strings.SplitN(line, "=", 2)
		if len(s) != 2 {
			log.Println("config line format incorrect:", line)
			return
		}
		key, value := s[0], s[1]
		value = strings.Trim(value, "\"")
		confs[key] = value
	}

	for k, v := range confs {
		switch k {
		case "NetpOpts":
			c.NetpOpts = strings.Fields(v)
		case "BinSuffix":
			setField(c, k, v)
		default:
			if strings.HasPrefix(k, "Netp") {
				setField(c, k, v)
			} else {
				// set env var
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	// build command line and env vars
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "/bin/bash"
	}

	binary := filepath.Join(c.NetpBins, c.NetpBase, c.BinSuffix)
	logFile := filepath.Join(c.NetpLogD, name, c.NetpLogF)

	env = append(env, "LD_LIBRARY_PATH="+c.NetpLibs)
	args := []string{name, "-logfile", logFile}
	args = append(args, c.NetpOpts...)
	cmd = exec.Command(binary, args...)

	return
}

func (c NetprobeComponent) run(name string, cmd *exec.Cmd, env []string) {
	wd := filepath.Join(c.NetpRoot, "netprobes", name)

	// actually run the process
	cmd.Env = append(os.Environ(), env...)
	err := cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	if cmd.Process != nil {
		// write pid file
		pidFile := filepath.Join(wd, "netprobe.pid")
		if file, err := os.Create(pidFile); err != nil {
			log.Printf("cannot open %q for writing", pidFile)
		} else {
			fmt.Fprintln(file, cmd.Process.Pid)
			file.Close()
		}
		// detach
		cmd.Process.Release()
	}
}

func (c NetprobeComponent) start(name string) {
	cmd, env := c.setup(name)
	if cmd == nil {
		return
	}

	if len(c.NetpUser) != 0 {
		u, _ := user.Current()
		if c.NetpUser != u.Username {
			log.Println("can't change user to", c.NetpUser)
			return
		}
	}

	c.run(name, cmd, env)
}

func (c NetprobeComponent) stop(name string) {
	pid, pidFile, err := getPid(Netprobe, c.NetpRoot, name)
	if err != nil {
		//		log.Println("cannot get PID for", name)
		return
	}

	// send sigterm
	log.Println("stopping netprobe", name, "with PID", pid)

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		log.Println("netprobe process not found, removing PID file")
		os.Remove(pidFile)
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
			log.Println("netprobe terminated")
			os.Remove(pidFile)
			return
		}
	}
	// sigkill
	if err = proc.Signal(syscall.SIGKILL); err != nil {
		log.Println("sending SIGKILL failed:", err)
		return
	}
	os.Remove(pidFile)
}

func (c NetprobeComponent) dir() string {
	return c.NetpRoot
}

func newNetprobe() (c NetprobeComponent) {
	// Bootstrap
	c.ITRSHome = itrsHome
	// empty slice
	c.NetpOpts = []string{}

	newComponent(&c)
	return
}
