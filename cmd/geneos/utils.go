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
	"reflect"
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
	return 0, fmt.Errorf("not found")
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
	// test euid here

	username := getString(c, Prefix(c)+"User")
	if len(username) == 0 {
		DebugLog.Println("no user configured")
		return true
	}
	u, _ := user.Current()

	return username == u.Username
}

// given a list of args (after command has been seen), check if first
// arg is a component type and depdup the names. A name of "all" will
// will override the rest and result in a lookup being done
//
// special case (shortcircuit) "config" ?
func parseArgs(args []string) (comp ComponentType, names []string) {
	if len(args) == 0 {
		return
	}

	comp = CompType(args[0])
	if comp == Unknown {
		// this may be a name or config option instead
		comp = None
		names = args
		return
	}

	// consume first arg and continue
	names = args[1:]

	// this doesn't work for all comp types - it adds all names
	// of all components and returns that
	/* 	for _, name := range names {
		if name == "all" {
			var confs []Component
			if comp == Unknown || comp == None {
				confs = allComponents()
			} else {
				confs = components(comp)
			}
			names = nil
			for _, c := range confs {
				names = append(names, Name(c))
			}
			break
		}
	} */

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

func getIntAsString(c interface{}, name string) string {
	v := reflect.ValueOf(c).Elem().FieldByName(Prefix(c) + name)
	if v.IsValid() && v.Kind() == reflect.Int {
		return fmt.Sprintf("%v", v.Int())
	}
	return ""
}

func getString(c interface{}, name string) string {
	v := reflect.ValueOf(c).Elem().FieldByName(name)
	if v.IsValid() && v.Kind() == reflect.String {
		return v.String()
	}
	return ""
}

func setField(c interface{}, k string, v string) {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}
	fv = fv.FieldByName(k)
	if fv.IsValid() && fv.CanSet() {
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(v)
		case reflect.Int:
			i, _ := strconv.Atoi(v)
			fv.SetInt(int64(i))
		default:
			log.Printf("cannot set %q to a %T\n", k, v)
		}
	} else {
		log.Println("cannot set", k)
	}
}

func setFieldSlice(c interface{}, k string, v []string) {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}
	fv = fv.FieldByName(k)
	if fv.IsValid() && fv.CanSet() {
		reflect.AppendSlice(fv, reflect.ValueOf(v))
		for _, val := range v {
			fv.Set(reflect.Append(fv, reflect.ValueOf(val)))
		}
	}
}
