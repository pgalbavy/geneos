//go:build linux

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
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
func findInstanceProc(c Instance) (pid int, st *syscall.Stat_t, err error) {
	var pids []int

	// safe to ignore error as it can only be bad pattern,
	// which means no matches to range over
	dirs, _ := filepath.Glob("/proc/[0-9]*")

	for _, dir := range dirs {
		pid, _ := strconv.Atoi(filepath.Base(dir))
		pids = append(pids, pid)
	}

	sort.Ints(pids)

	for _, pid = range pids {
		var data []byte
		data, err = os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			continue
		}
		args := bytes.Split(data, []byte("\000"))
		bin := filepath.Base(string(args[0]))
		if strings.HasPrefix(bin, Type(c).String()) {
			for _, arg := range args[1:] {
				if string(arg) == Name(c) {
					if s, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err == nil {
						st = s.Sys().(*syscall.Stat_t)
					}
					return
				}
			}
		}
	}
	return 0, nil, ErrProcNotExist
}

func getUser(username string) (uid, gid int, gids []uint32, err error) {
	uid = -1
	gid = -1

	if username == "" {
		username = RunningConfig.DefaultUser
	}

	u, err := user.Lookup(username)
	if err != nil {
		return
	}
	uid, _ = strconv.Atoi(u.Uid)
	gid, _ = strconv.Atoi(u.Gid)
	groups, _ := u.GroupIds()
	for _, g := range groups {
		gid, _ := strconv.Atoi(g)
		gids = append(gids, uint32(gid))
	}
	return
}

//
// set-up the Cmd to set uid, gid and groups of the username given
// Note: does not change stdout etc. which is done later
//
func setuser(cmd *exec.Cmd, username string) (err error) {
	uid, gid, gids, err := getUser(username)
	if err != nil {
		return
	}

	// do not set-up credentials if no-change
	if os.Getuid() == uid {
		return nil
	}

	// no point continuing if not root
	if !superuser {
		return ErrPermission
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
//
func canControl(c Instance) bool {
	if superuser {
		DebugLog.Println("I am root")
		return true
	}

	username := getString(c, Prefix(c)+"User")
	if len(username) == 0 {
		DebugLog.Println("no user configured")
		// assume the caller with try to set-up the correct user
		return true
	}

	u, err := user.Lookup(username)
	if err != nil {
		// user not found, should fails
		return false
	}

	uid, _ := strconv.Atoi(u.Uid)
	if uid == os.Getuid() || uid == os.Geteuid() {
		// if uid != euid then child proc may fail because
		// of linux ld.so secure-execution discarding
		// envs like LD_LIBRARY_PATH, account for this?
		return true
	}

	uc, _ := user.Current()
	return username == uc.Username
}

// given a list of args (after command has been seen), check if first
// arg is a component type and depdup the names. A name of "all" will
// will override the rest and result in a lookup being done
//
// Check for spaces in args - they would have come in in quotes - and
// do something with them.
//
// Check for specific, configurable, character (default '+') and replace with space
// e.g. Demo+Gateway -> "Demo Gateway"
//
// args with an '=' should be checked and only allowed if there are names?
//
// support glob style wildcards for instance names - allow through, let loopCommand*
// deal with them
//
func checkComponentArg(rawargs []string) (ct ComponentType, args []string, params []string) {
	if len(rawargs) == 0 {
		// wildcard everything
		ct = None
	} else if ct = parseComponentName(rawargs[0]); ct == Unknown {
		// first arg is not a known type
		ct = None
		args = rawargs
	} else {
		args = rawargs[1:]
	}

	return
}

func parseArgs(rawargs []string) (ct ComponentType, args []string, params []string) {
	if len(rawargs) == 0 {
		// wildcard everything
		ct = None
	} else if ct = parseComponentName(rawargs[0]); ct == Unknown {
		// first arg is not a known type
		ct = None
		args = rawargs
	} else {
		args = rawargs[1:]
	}

	// empty list of names = all names for that ct
	if len(args) == 0 {
		args = emptyArgs(ct)
	}

	// make sure names/args are unique but retain order
	// check for reserved names here?
	// do space exchange inbound here
	var newnames []string

	m := make(map[string]bool, len(args))
	for _, name := range args {
		// filter name here
		if reservedName(args[0]) {
			log.Fatalf("%q is reserved instance name", args[0])
		}
		if !validInstanceName(args[0]) {
			DebugLog.Printf("%q is not a valid instance name", args[0])
			break
		}
		if m[name] {
			continue
		}
		newnames = append(newnames, name)
		args = args[1:]
		m[name] = true
	}
	params = args
	args = newnames

	// repeat if args is now empty (all params)
	if len(args) == 0 {
		args = emptyArgs(ct)
	}

	DebugLog.Println("params:", params)
	return
}

func emptyArgs(ct ComponentType) (args []string) {
	var confs []Instance
	switch ct {
	case None, Unknown:
		// wildcard again - sort oder matters, fix
		confs = allInstances()
	default:
		confs = instances(ct)
	}
	for _, c := range confs {
		args = append(args, Name(c))
	}
	return
}

// seperate reserved words and invalid syntax
//
func reservedName(in string) (ok bool) {
	DebugLog.Printf("checking %q", in)
	if parseComponentName(in) != Unknown {
		DebugLog.Println("matches a reserved word")
		return true
	}
	if RunningConfig.ReservedNames != "" {
		list := strings.Split(in, string(os.PathListSeparator))
		for _, n := range list {
			if strings.EqualFold(in, n) {
				DebugLog.Println("matches a user defined reserved name")
				return true
			}
		}
	}
	return
}

// spaces are valid - dumb, but valid - for now
var validStringRE = regexp.MustCompile(`^\w[\w -]*$`)

// return true while a string is considered a valid instance name
//
// used to consume instance names until parameters are then passed down
//
func validInstanceName(in string) (ok bool) {
	DebugLog.Printf("checking %q", in)
	ok = validStringRE.MatchString(in)
	DebugLog.Println("rexexp match", ok)
	return
}

func spaceToReplace(in string) string {
	return strings.ReplaceAll(in, " ", RunningConfig.ReplaceSpace)
}

func replaceToSpace(in string) string {
	return strings.ReplaceAll(in, RunningConfig.ReplaceSpace, " ")
}

type perComponentFuncs map[ComponentType]func(Instance, []string) error

func loopCommandMap(funcs perComponentFuncs, ct ComponentType, args []string, params []string) (err error) {
	for _, name := range args {
		for _, c := range NewComponent(ct, name) {
			if err = loadConfig(c, false); err != nil {
				log.Println(Type(c), Name(c), "cannot load configuration")
				return
			}

			if fn, ok := funcs[Type(c)]; ok {
				if err = fn(c, params); err != nil {
					log.Println(Type(c), Name(c), err)
				}
			}
		}
	}
	return nil
}

// given a compoent type and a slice of args, call the function for each arg
//
// reply on NewComponent() checking the component type and returning a slice
// of all matching components for a single name in an arg (e.g all instances
// called 'thisserver')
func loopCommand(fn func(Instance, []string) error, ct ComponentType, args []string, params []string) (err error) {
	for _, name := range args {
		for _, c := range NewComponent(ct, name) {
			if err = loadConfig(c, false); err != nil {
				log.Println(Type(c), Name(c), "cannot load configuration")
				return
			}
			if err = fn(c, params); err != nil {
				log.Println(Type(c), Name(c), err)
			}
		}
	}
	return nil
}

// like the above but only process the first arg (if any) to allow for those commands
// that accept only zero or one named instance and the rest of the args are parameters
// pass the remaining args the to function
func singleCommand(fn func(Instance, []string, []string) error, ct ComponentType, args []string, params []string) (err error) {
	if len(args) == 0 {
		// do nothing
		return
	}
	name := args[0]
	for _, c := range NewComponent(ct, name) {
		if err = loadConfig(c, false); err != nil {
			log.Println(Type(c), Name(c), "cannot load configuration")
			return
		}

		if err = fn(c, args[1:], params); err != nil {
			log.Println(Type(c), Name(c), err)
		}
	}
	return nil
}

func cleanRelativePath(path string) (clean string, err error) {
	path = strings.TrimSuffix(path, string(filepath.Separator))
	p := strings.Split(path, string(filepath.Separator))
	if len(p) > 0 && len(p[0]) == 0 {
		DebugLog.Println("dest path must be relative")
		return path, ErrInvalidArgs
	}
	for _, e := range p {
		if e == ".." {
			DebugLog.Println("dest path cannot contain '..'")
			return path, ErrInvalidArgs
		}
	}
	clean = filepath.Clean(path)

	return
}

func removePathList(c Instance, paths string) (err error) {
	list := filepath.SplitList(paths)
	for _, p := range list {
		// clean path, error on absolute or parent paths, like 'upload'
		// walk globbed directories, remove everything
		p, err = cleanRelativePath(p)
		if err != nil {
			log.Fatalln(p, err)
		}
		// glob here
		m, err := filepath.Glob(filepath.Join(Home(c), p))
		if err != nil {
			log.Fatalln(err)
		}
		for _, f := range m {
			if err = os.RemoveAll(f); err != nil {
				log.Fatalln(err)
			}
		}
	}
	return
}

// logdir = Logd relative to Home or absolute
func getLogfilePath(c Instance) (logdir string) {
	logd := filepath.Clean(getString(c, Prefix(c)+"Logd"))
	switch {
	case logd == "":
		logdir = Home(c)
	case strings.HasPrefix(logd, string(os.PathSeparator)):
		logdir = logd
	default:
		logdir = filepath.Join(Home(c), logd)
	}
	logdir = filepath.Join(logdir, getString(c, Prefix(c)+"LogF"))
	return
}

// reflect methods to get and set struct fields

func getIntAsString(c interface{}, name string) string {
	v := reflect.ValueOf(c).Elem().FieldByName(name)
	if v.IsValid() && v.Kind() == reflect.Int {
		return fmt.Sprintf("%v", v.Int())
	}
	return ""
}

func getString(c interface{}, name string) string {
	v := reflect.ValueOf(c)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	if !v.IsValid() || v.Kind() != reflect.Struct {
		return ""
	}

	v = v.FieldByName(name)

	if v.IsValid() && v.Kind() == reflect.String {
		return v.String()
	}

	return ""
}

func getSliceStrings(c interface{}, name string) (strings []string) {
	v := reflect.ValueOf(c)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil
	}

	v = v.FieldByName(name)

	if v.Type() != reflect.TypeOf(strings) {
		return nil
	}

	return v.Interface().([]string)
}

func setField(c interface{}, k string, v string) (err error) {
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
			return fmt.Errorf("cannot set %q to a %T: %w", k, v, ErrInvalidArgs)
		}
	} else {
		return fmt.Errorf("cannot set %q: %w", k, ErrInvalidArgs)
	}
	return
}

func setFieldSlice(c interface{}, k string, v []string) (err error) {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}
	fv = fv.FieldByName(k)
	if fv.IsValid() && fv.CanSet() {
		fv.Set(reflect.ValueOf(v))
	}
	return
}

func slicetoi(s []string) (n []int) {
	for _, x := range s {
		i, err := strconv.Atoi(x)
		if err != nil {
			i = 0
		}
		n = append(n, i)
	}
	return
}
