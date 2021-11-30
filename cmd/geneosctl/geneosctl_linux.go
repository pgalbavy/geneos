//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var itrsHome string = "/opt/itrs"

func Stop(c Component) (err error) {
	pid, err := findProc(c)
	if err != nil {
		return
	}

	// who is the process running as?
	s, _ := os.Stat(fmt.Sprintf("/proc/%d", pid))
	st := s.Sys().(*syscall.Stat_t)
	log.Println("process running as", st.Uid, st.Gid)

	proc, _ := os.FindProcess(pid)

	if canControl(c) {
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
				return nil
			}
		}
		// sigkill
		if err = proc.Signal(syscall.SIGKILL); err != nil {
			log.Println("sending SIGKILL failed:", err)
			return
		}
	} else {
		err = fmt.Errorf("cannot stop %s %s process, no permission (%v != %v)", Type(c), Name(c), st.Uid, os.Getuid())
	}
	return
}

func Start(c Component) (err error) {
	cmd, env := Command(c)
	if cmd == nil {
		return
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
		log.Fatalf("cannot open %q: %s\n", errfile, err)
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
	DebugLogger.Println("started process", cmd.Process.Pid)

	if cmd.Process != nil {
		// detach from control
		cmd.Process.Release()
	}

	return
}

func Reload(c Component) (err error) {
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

// locate a process by compoent type and name
//
// the component type must be part of the basename of the executable and
// the component name must be on the command line as an exact and standalone
// args
//
func findProc(c Component) (int, error) {
	var pids []int

	DebugLogger.Println("looking for", Type(c), Name(c))
	// safe to ignore error as it can only be bad pattern,
	// which means no matches to range over
	dirs, _ := filepath.Glob("/proc/[0-9]*")

	for _, dir := range dirs {
		pid, _ := strconv.Atoi(filepath.Base(dir))
		pids = append(pids, pid)
	}

	sort.Ints(pids)

	for _, pid := range pids {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			if !errors.Is(err, fs.ErrPermission) {
				DebugLogger.Printf("reading %q failed, err: %q\n", pid, err)
			}
			// an error can be transient, just debug and ignore
			continue
		}
		args := bytes.Split(data, []byte("\000"))
		bin := filepath.Base(string(args[0]))
		if strings.HasPrefix(bin, Type(c).String()) {
			for _, arg := range args[1:] {
				if string(arg) == Name(c) {
					DebugLogger.Println(pid, "matches", bin)
					return pid, nil
				}
			}
		}
	}
	return 0, fmt.Errorf(Type(c).String(), Name(c), "not found")
}

// set-up the Cmd to set uid, gid and groups of the username given
// does not change stdout etc.
func setuid(cmd *exec.Cmd, username string) error {
	var gids []uint32

	if os.Geteuid() != 0 && os.Getuid() != 0 {
		return fmt.Errorf("not running as root")
	}

	u, err := user.Lookup(username)
	if err != nil {
		fmt.Println("lookup:", err)
		return err
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	groups, _ := u.GroupIds()
	for _, g := range groups {
		gid, _ := strconv.Atoi(g)
		gids = append(gids, uint32(gid))
	}

	cred := &syscall.Credential{
		Uid:         uint32(uid),
		Gid:         uint32(gid),
		Groups:      gids,
		NoSetGroups: false,
	}
	sys := &syscall.SysProcAttr{Credential: cred}

	cmd.SysProcAttr = sys

	return nil
}

// check if the current user can do "something" with the selected component
//
// just check if running as root or if a username is specified in the config
// that the current user matches.
//
// this does not however change the user to match anything, so starting a
// process still requires a seteuid type change
func canControl(c Component) bool {
	if superuser {
		DebugLogger.Println("am root")
		return true
	}
	username := getString(c, Prefix(c)+"User")
	if len(username) == 0 {
		DebugLogger.Println("no user configured")
		return true
	}
	u, _ := user.Current()

	return username == u.Username
}
