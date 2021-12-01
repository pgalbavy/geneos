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
)

// locate a process by compoent type and name
//
// the component type must be part of the basename of the executable and
// the component name must be on the command line as an exact and standalone
// args
//
func findProc(c Component) (int, error) {
	var pids []int

	DebugLog.Println("looking for", Type(c), Name(c))
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
				DebugLog.Printf("reading %q failed, err: %q\n", pid, err)
			}
			// an error can be transient, just debug and ignore
			continue
		}
		args := bytes.Split(data, []byte("\000"))
		bin := filepath.Base(string(args[0]))
		if strings.HasPrefix(bin, Type(c).String()) {
			for _, arg := range args[1:] {
				if string(arg) == Name(c) {
					DebugLog.Println(pid, "matches", bin)
					return pid, nil
				}
			}
		}
	}
	return 0, fmt.Errorf(Type(c).String(), Name(c), "not found")
}

// set-up the Cmd to set uid, gid and groups of the username given
// does not change stdout etc.
//
// also allow for euid = wanted user, not just root
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
		DebugLog.Println("am root")
		return true
	}
	username := getString(c, Prefix(c)+"User")
	if len(username) == 0 {
		DebugLog.Println("no user configured")
		return true
	}
	u, _ := user.Current()

	return username == u.Username
}

// given a list of args (after command has been see), check if first
// arg is a component type and
func parseArgs(args []string) (comp ComponentType, names []string) {
	if len(args) == 0 {
		return
	}

	comp = CompType(args[0])
	if comp == None {
		// this may be a name instead
		// and we might wildcard the component
		names = args
	} else {
		names = args[1:]
	}

	if len(names) > 1 {
		// make sure names are unique
		m := make(map[string]bool, len(names))
		for _, name := range names {
			m[name] = true
		}
		names = nil
		for name := range m {
			names = append(names, name)
		}
	}
	return
}
