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

type GatewayComponent struct {
	Component
	ITRSHome  string
	GateRoot  string `default:"{{join .ITRSHome \"gateway\"}}"`
	GateBins  string `default:"{{join .ITRSHome \"packages\" \"gateway\"}}"`
	GateBase  string `default:"active_prod"`
	GateLogD  string `default:"{{join .GateRoot \"gateways\"}}"`
	GateLogF  string `default:"gateway.log"`
	GateMode  string `default:"background"`
	GateLicP  string `default:"7041"`
	GateLicH  string `default:"localhost"`
	GateOpts  []string
	GateLibs  string `default:"{{join .GateBins .GateBase \"lib64\"}}:/usr/lib64"`
	GateUser  string
	BinSuffix string `default:"gateway2.linux_64"`
}

func (c GatewayComponent) all() []string {
	dir := filepath.Join(c.GateRoot, "gateways")
	return dirs(dir)
}

/*
func (c *GatewayComponent) setField(k, v string) {
	fv := reflect.ValueOf(c).Elem().FieldByName(k)
	if fv.IsValid() {
		fv.SetString(v)
	}

}
*/
func (c GatewayComponent) setup(name string) (cmd *exec.Cmd, env []string) {
	wd := filepath.Join(c.GateRoot, "gateways", name)
	if err := os.Chdir(wd); err != nil {
		log.Println("cannot chdir() to", wd)
		return
	}
	rcFile, err := os.Open("gateway.rc")
	if err != nil {
		log.Println("cannot open gateway.rc")
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
		case "GateOpts":
			c.GateOpts = strings.Fields(v)
		case "BinSuffix":
			setField(c, k, v)
		default:
			if strings.HasPrefix(k, "Gate") {
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

	binary := filepath.Join(c.GateBins, c.GateBase, c.BinSuffix)
	logFile := filepath.Join(c.GateLogD, name, c.GateLogF)

	env = append(env, "LD_LIBRARY_PATH="+c.GateLibs)
	args := []string{name, "-logfile", logFile}
	args = append(args, c.GateOpts...)
	cmd = exec.Command(binary, args...)

	return
}

func (c GatewayComponent) run(name string, cmd *exec.Cmd, env []string) {
	wd := filepath.Join(c.GateRoot, "gateways", name)

	// actually run the process
	cmd.Env = append(os.Environ(), env...)
	err := cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	if cmd.Process != nil {
		// write pid file
		pidFile := filepath.Join(wd, "gateway.pid")
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

func (c GatewayComponent) start(name string) {
	cmd, env := c.setup(name)
	if cmd == nil {
		return
	}

	if len(c.GateUser) != 0 {
		u, _ := user.Current()
		if c.GateUser != u.Username {
			log.Println("can't change user to", c.GateUser)
			return
		}
	}

	c.run(name, cmd, env)
}

func (c GatewayComponent) stop(name string) {
	pid, pidFile, err := getPid(Gateway, c.GateRoot, name)
	if err != nil {
		//		log.Println("cannot get PID for", name)
		return
	}

	// send sigterm
	log.Println("stopping gateway", name, "with PID", pid)

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		log.Println("gateway process not found, removing PID file")
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
			log.Println("gateway terminated")
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

func (c GatewayComponent) dir() string {
	return c.GateRoot
}

func newGateway() (c GatewayComponent) {
	// Bootstrap
	c.ITRSHome = itrsHome
	// empty slice
	c.GateOpts = []string{}

	newComponent(&c)
	return
}
