//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/template"
)

// walk the /proc directory (local or remote) and find the matching pid
// this is subject to races, but...
func findInstancePID(c Instances) (pid int, err error) {
	var pids []int

	// safe to ignore error as it can only be bad pattern,
	// which means no matches to range over
	dirs, _ := globPath(c.Location(), "/proc/[0-9]*")

	for _, dir := range dirs {
		p, _ := strconv.Atoi(filepath.Base(dir))
		pids = append(pids, p)
	}

	sort.Ints(pids)

	for _, pid = range pids {
		var data []byte
		data, err = readFile(c.Location(), fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			// process may disappear by this point, ignore error
			logDebug.Println(err)
			continue
		}
		args := bytes.Split(data, []byte("\000"))
		execfile := filepath.Base(string(args[0]))
		switch c.Type() {
		case Webserver:
			var wdOK, jarOK bool
			if execfile != "java" {
				continue
			}
			for _, arg := range args[1:] {
				if string(arg) == "-Dworking.directory="+c.Home() {
					wdOK = true
				}
				if strings.HasSuffix(string(arg), "geneos-web-server.jar") {
					jarOK = true
				}
				if wdOK && jarOK {
					return
				}
			}
		default:
			if strings.HasPrefix(execfile, c.Type().String()) {
				for _, arg := range args[1:] {
					// very simplistic - we look for a bare arg that matches the instance name
					if string(arg) == c.Name() {
						// found
						return
					}
				}
			}
		}
	}
	return 0, ErrProcNotExist
}

// locate a process by compoent type and name
//
// the component type must be part of the basename of the executable and
// the component name must be on the command line as an exact and
// standalone args
//
func findInstanceProc(c Instances) (pid int, uid uint32, gid uint32, mtime int64, err error) {
	pid, err = findInstancePID(c)
	if err == nil {
		var s fileStat
		s, err = statFile(c.Location(), fmt.Sprintf("/proc/%d", pid))
		return pid, s.uid, s.uid, s.mtime, err
	}
	return 0, 0, 0, 0, ErrProcNotExist
}

func getUser(username string) (uid, gid uint32, gids []uint32, err error) {
	uid, gid = math.MaxUint32, math.MaxUint32

	if username == "" {
		username = RunningConfig.DefaultUser
	}

	u, err := user.Lookup(username)
	if err != nil {
		return
	}
	ux, err := strconv.ParseInt(u.Uid, 10, 32)
	if err != nil || ux < 0 || ux > math.MaxUint32 {
		logError.Fatalln("uid out of range:", u.Uid)
	}
	uid = uint32(ux)
	gx, err := strconv.ParseInt(u.Gid, 10, 32)
	if err != nil || gx < 0 || gx > math.MaxUint32 {
		logError.Fatalln("gid out of range:", u.Gid)
	}
	gid = uint32(gx)
	groups, _ := u.GroupIds()
	for _, g := range groups {
		gid, err := strconv.ParseInt(g, 10, 32)
		if err != nil || gid < 0 || gid > math.MaxUint32 {
			logError.Fatalln("gid out of range:", g)
		}
		gids = append(gids, uint32(gid))
	}
	return
}

//
// set-up the Cmd to set uid, gid and groups of the username given
// Note: does not change stdout etc. which is done later
//
func setUser(cmd *exec.Cmd, username string) (err error) {
	uid, gid, gids, err := getUser(username)
	if err != nil {
		return
	}

	// do not set-up credentials if no-change
	if os.Getuid() == int(uid) {
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
func canControl(c Instances) bool {
	if superuser {
		logDebug.Println("I am root")
		return true
	}

	username := getString(c, c.Prefix("User"))
	if len(username) == 0 {
		logDebug.Println("no user configured")
		// assume the caller with try to set-up the correct user
		return true
	}

	u, err := user.Lookup(username)
	if err != nil {
		// user not found, should fail
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
func checkComponentArg(rawargs []string) (ct Component, args []string, params []string) {
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

func parseArgs(rawargs []string) (ct Component, args []string, params []string) {
	if len(rawargs) == 0 {
		// no more arguments? wildcard everything
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
		args = ct.allArgsForComponent()
	}

	logDebug.Println("ct, args, params", ct, args, params)

	// make sure names/args are unique but retain order
	// check for reserved names here?
	// do space exchange inbound here
	var newnames []string

	m := make(map[string]bool, len(args))
	for _, name := range args {
		// filter name here
		if reservedName(args[0]) {
			logError.Fatalf("%q is reserved instance name", args[0])
		}
		if !validInstanceName(args[0]) {
			logDebug.Printf("%q is not a valid instance name", args[0])
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
		args = ct.allArgsForComponent()
	}

	logDebug.Println("params:", params)
	return
}

// for commands (like 'add') that don't want to know about existing matches
func parseArgsNoWildcard(rawargs []string) (ct Component, args []string, params []string) {
	var newnames []string

	if len(rawargs) == 0 {
		return
	}
	if ct = parseComponentName(rawargs[0]); ct == Unknown {
		return
	}
	args = rawargs[1:]

	logDebug.Println("ct, args, params", ct, args, params)

	m := make(map[string]bool, len(args))
	for _, name := range args {
		// filter name here
		if reservedName(args[0]) {
			logError.Fatalf("%q is reserved instance name", args[0])
		}
		if !validInstanceName(args[0]) {
			logDebug.Printf("%q is not a valid instance name", args[0])
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

	logDebug.Println("params:", params)
	return
}

func (ct Component) allArgsForComponent() (args []string) {
	var confs []Instances
	switch ct {
	case None, Unknown:
		// wildcard again - sort oder matters, fix
		confs = allInstances(ALL)
	case Remote:
		// we only look for actual "remote" components on the local system
		confs = append(confs, Remote.remoteInstances(LOCAL)...)
	default:
		// scan all remotes for the instances of type ct
		for _, remote := range allRemotes() {
			logDebug.Println("checking remote:", remote.Name())
			confs = append(confs, ct.remoteInstances(remote.Name())...)
		}
	}
	for _, c := range confs {
		// XXX
		args = append(args, c.Name()+"@"+c.Location())
	}
	return
}

// seperate reserved words and invalid syntax
//
func reservedName(in string) (ok bool) {
	logDebug.Printf("checking %q", in)
	if parseComponentName(in) != Unknown {
		logDebug.Println("matches a reserved word")
		return true
	}
	if RunningConfig.ReservedNames != "" {
		list := strings.Split(in, string(os.PathListSeparator))
		for _, n := range list {
			if strings.EqualFold(in, n) {
				logDebug.Println("matches a user defined reserved name")
				return true
			}
		}
	}
	return
}

// spaces are valid - dumb, but valid - for now
var validStringRE = regexp.MustCompile(`^\w[@\w -]*$`)

// return true while a string is considered a valid instance name
//
// used to consume instance names until parameters are then passed down
//
func validInstanceName(in string) (ok bool) {
	ok = validStringRE.MatchString(in)
	if !ok {
		logDebug.Println("no rexexp match:", in)
	}
	return
}

// given a component type and a slice of args, call the function for each arg
//
// rely on NewComponent() checking the component type and returning a slice
// of all matching components for a single name in an arg (e.g all instances
// called 'thisserver')
//
// try to use go routines here - mutexes required
func loopCommand(fn func(Instances, []string) error, ct Component, args []string, params []string) (err error) {
	for _, name := range args {
		for _, c := range ct.Match(name) {
			if err = loadConfig(c, false); err != nil {
				log.Println(c.Type(), c.Name(), "cannot load configuration")
				return
			}
			if err = fn(c, params); err != nil && !errors.Is(err, ErrProcNotExist) {
				log.Println(c.Type(), c.Name(), err)
			}
		}
	}
	return nil
}

// call a function but with an extra subcommand parameter to allow some indirection
func loopSubcommand(fn func(Instances, string, []string) error, subcommand string, ct Component, args []string, params []string) (err error) {
	for _, name := range args {
		for _, c := range ct.Match(name) {
			if err = loadConfig(c, false); err != nil {
				log.Println(c.Type(), c.Name(), "cannot load configuration")
				return
			}
			if err = fn(c, subcommand, params); err != nil && !errors.Is(err, ErrProcNotExist) {
				log.Println(c.Type(), c.Name(), err)
			}
		}
	}
	return nil
}

// like the above but only process the first arg (if any) to allow for those commands
// that accept only zero or one named instance and the rest of the args are parameters
// pass the remaining args the to function
//
func (ct Component) singleCommand(fn func(Instances, []string, []string) error, args []string, params []string) (err error) {
	if len(args) == 0 {
		// do nothing
		return
	}
	name := args[0]
	for _, c := range ct.Match(name) {
		if err = loadConfig(c, false); err != nil {
			log.Println(c.Type(), c.Name(), "cannot load configuration")
			return
		}

		// empty remaining args, prepend to params
		if err = fn(c, []string{}, append(args[1:], params...)); err != nil {
			log.Println(c.Type(), c.Name(), err)
		}
	}
	return nil
}

func filenameFromHTTPResp(resp *http.Response, u *url.URL) (filename string, err error) {
	cd, ok := resp.Header[http.CanonicalHeaderKey("content-disposition")]
	if !ok && resp.Request.Response != nil {
		cd, ok = resp.Request.Response.Header[http.CanonicalHeaderKey("content-disposition")]
	}
	if ok {
		_, params, err := mime.ParseMediaType(cd[0])
		if err == nil {
			if f, ok := params["filename"]; ok {
				filename = f
			}
		}
	}

	// if no content-disposition, then grab the path from the response URL
	if filename == "" {
		filename, err = cleanRelativePath(path.Base(u.Path))
		if err != nil {
			return
		}
	}
	return
}

func cleanRelativePath(path string) (clean string, err error) {
	clean = filepath.Clean(path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "../") {
		logDebug.Printf("path %q must be relative and descending only", clean)
		return "", ErrInvalidArgs
	}

	return
}

// given a filename or path, prepend the instance home directory
// if not absolute, and clean
func filepathForInstance(c Instances, file string) (path string) {
	path = filepath.Clean(file)
	if filepath.IsAbs(path) {
		return
	}
	return filepath.Join(c.Home(), path)
}

func removePathList(c Instances, paths string) (err error) {
	list := filepath.SplitList(paths)
	for _, p := range list {
		// clean path, error on absolute or parent paths, like 'upload'
		// walk globbed directories, remove everything
		p, err = cleanRelativePath(p)
		if err != nil {
			return fmt.Errorf("%s %w", p, err)
		}
		// glob here
		m, err := globPath(c.Location(), filepath.Join(c.Home(), p))
		if err != nil {
			return err
		}
		for _, f := range m {
			if err = removeAll(c.Location(), f); err != nil {
				logError.Println(err)
				continue
			}
		}
	}
	return
}

// logdir = LogD relative to Home or absolute
func getLogfilePath(c Instances) (logdir string) {
	logd := filepath.Clean(getString(c, c.Prefix("LogD")))
	switch {
	case logd == "":
		logdir = c.Home()
	case strings.HasPrefix(logd, string(os.PathSeparator)):
		logdir = logd
	default:
		logdir = filepath.Join(c.Home(), logd)
	}
	logdir = filepath.Join(logdir, getString(c, c.Prefix("LogF")))
	return
}

// reflect methods to get and set struct fields

func getIntAsString(c interface{}, name string) string {
	v := reflect.ValueOf(c)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	if !v.IsValid() || v.Kind() != reflect.Struct {
		return ""
	}

	v = v.FieldByName(name)
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
		return fmt.Errorf("cannot set %q: %w (isValid=%v, canset=%v, type=%v)", k, ErrInvalidArgs, fv.IsValid(), fv.CanSet(), fv.Type())
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

func readSource(source string) (b []byte, err error) {
	var from io.ReadCloser

	u, err := url.Parse(source)
	if err != nil {
		return
	}

	switch {
	case u.Scheme == "https" || u.Scheme == "http":
		resp, err := http.Get(u.String())
		if err != nil {
			return nil, err
		}

		from = resp.Body
		defer from.Close()

	case source == "-":
		from = os.Stdin
		source = "STDIN"
		defer from.Close()

	default:
		from, err = os.Open(source)
		if err != nil {
			return
		}
		defer from.Close()
	}

	b, err = io.ReadAll(from)
	return
}

// a template function to support "{{join .X .Y}}"
var textJoinFuncs = template.FuncMap{"join": filepath.Join}

// setDefaults() is a common function called by component New*()
// functions to iterate over the component specific instance
// struct and set the defaults as defined in the 'defaults'
// struct tags.
func setDefaults(c interface{}) (err error) {
	st := reflect.TypeOf(c)
	sv := reflect.ValueOf(c)
	for st.Kind() == reflect.Ptr || st.Kind() == reflect.Interface {
		st = st.Elem()
		sv = sv.Elem()
	}

	n := sv.NumField()

	for i := 0; i < n; i++ {
		ft := st.Field(i)
		fv := sv.Field(i)

		// only set plain strings
		if !fv.CanSet() {
			continue
		}
		if def, ok := ft.Tag.Lookup("default"); ok {
			// treat all defaults as if they are templates
			val, err := template.New(ft.Name).Funcs(textJoinFuncs).Parse(def)
			if err != nil {
				log.Println("parse error:", def)
				continue
			}
			var b bytes.Buffer
			if err = val.Execute(&b, c); err != nil {
				log.Println("cannot convert:", def)
			}
			if err = setField(c, ft.Name, b.String()); err != nil {
				return err
			}
		}
	}
	return
}

// Given a slice of directory entries, sort in place
func sortDirEntries(files []fs.DirEntry) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})
}

// Return a sorted list of sub-directories
func sortedInstancesInDir(remote string, dir string) []string {
	files, _ := readDir(remote, dir)
	sortDirEntries(files)
	components := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			components = append(components, file.Name()+"@"+remote)
		}
	}
	return components
}
