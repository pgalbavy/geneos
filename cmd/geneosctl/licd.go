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

type LicdComponent struct {
	Component
	ITRSHome  string
	LicdRoot  string `default:"{{join .ITRSHome \"licd\"}}"`
	LicdBins  string `default:"{{join .ITRSHome \"packages\" \"licd\"}}"`
	LicdBase  string `default:"active_prod"`
	LicdLogD  string `default:"{{join .LicdRoot \"licds\"}}"`
	LicdLogF  string `default:"licd.log"`
	LicdMode  string `default:"background"`
	LicdOpts  []string
	LicdLibs  string `default:"{{join .LicdBins .LicdBase \"lib64\"}}"`
	LicdUser  string
	BinSuffix string `default:"licd.linux_64"`
}

func (c LicdComponent) all() []string {
	dir := filepath.Join(c.LicdRoot, "licds")
	return dirs(dir)
}

/* func (c *LicdComponent) setField(k, v string) {
	fv := reflect.ValueOf(c).Elem().FieldByName(k)
	if fv.IsValid() {
		fv.SetString(v)
	}
}
*/
func (c LicdComponent) setup(name string) (cmd *exec.Cmd, env []string) {
	wd := filepath.Join(c.LicdRoot, "licds", name)
	if err := os.Chdir(wd); err != nil {
		log.Println("cannot chdir() to", wd)
		return
	}
	rcFile, err := os.Open("licd.rc")
	if err != nil {
		log.Println("cannot open licd.rc")
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
		case "LicdOpts":
			c.LicdOpts = strings.Fields(v)
		case "BinSuffix":
			setField(c, k, v)
		default:
			if strings.HasPrefix(k, "Licd") {
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

	binary := filepath.Join(c.LicdBins, c.LicdBase, c.BinSuffix)
	logFile := filepath.Join(c.LicdLogD, name, c.LicdLogF)

	env = append(env, "LD_LIBRARY_PATH="+c.LicdLibs)
	args := []string{name, "-logfile", logFile}
	args = append(args, c.LicdOpts...)
	cmd = exec.Command(binary, args...)

	return
}

func (c LicdComponent) run(name string, cmd *exec.Cmd, env []string) {
	wd := filepath.Join(c.LicdRoot, "licds", name)

	// actually run the process
	cmd.Env = append(os.Environ(), env...)
	err := cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	if cmd.Process != nil {
		// write pid file
		pidFile := filepath.Join(wd, "licd.pid")
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

func (c LicdComponent) dir() string {
	return c.LicdRoot
}

func (c LicdComponent) start(name string) {
	cmd, env := c.setup(name)
	if cmd == nil {
		return
	}

	if len(c.LicdUser) != 0 {
		u, _ := user.Current()
		if c.LicdUser != u.Username {
			log.Println("can't change user to", c.LicdUser)
			return
		}
	}

	c.run(name, cmd, env)
}

func (c LicdComponent) stop(name string) {
	pid, pidFile, err := getPid(Licd, c.dir(), name)
	if err != nil {
		//		log.Println("cannot get PID for", name)
		return
	}

	// send sigterm
	log.Println("stopping licd", name, "with PID", pid)

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		log.Println("licd process not found, removing PID file")
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
			log.Println("licd terminated")
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

func newLicd() (c LicdComponent) {
	// Bootstrap
	c.ITRSHome = itrsHome
	// empty slice
	c.LicdOpts = []string{}

	newComponent(&c)
	return
}
