//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/template"
)

// locate a process instance
//
// the component type must be part of the basename of the executable and
// the component name must be on the command line as an exact and
// standalone args
//
// walk the /proc directory (local or remote) and find the matching pid
// this is subject to races, but...
func findInstancePID(c Instances) (pid int, err error) {
	var pids []int

	// safe to ignore error as it can only be bad pattern,
	// which means no matches to range over
	dirs, _ := c.Remote().globPath("/proc/[0-9]*")

	for _, dir := range dirs {
		p, _ := strconv.Atoi(filepath.Base(dir))
		pids = append(pids, p)
	}

	sort.Ints(pids)

	binsuffix := getString(c, "BinSuffix")

	for _, pid = range pids {
		var data []byte
		data, err = c.Remote().readFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			// process may disappear by this point, ignore error
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
			if strings.HasPrefix(execfile, binsuffix) {
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

func findInstanceProc(c Instances) (pid int, uid uint32, gid uint32, mtime int64, err error) {
	pid, err = findInstancePID(c)
	if err == nil {
		var s fileStat
		s, err = c.Remote().statFile(fmt.Sprintf("/proc/%d", pid))
		return pid, s.uid, s.gid, s.mtime, err
	}
	return 0, 0, 0, 0, ErrProcNotExist
}

func getUser(username string) (uid, gid int, gids []int, err error) {
	uid, gid = math.MaxUint32, math.MaxUint32

	if username == "" {
		username = GlobalConfig["DefaultUser"]
	}

	u, err := user.Lookup(username)
	if err != nil {
		return -1, -1, nil, err
	}
	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		uid = -1
	}

	gid, err = strconv.Atoi(u.Gid)
	if err != nil {
		gid = -1
	}
	groups, _ := u.GroupIds()
	for _, g := range groups {
		gid, err := strconv.Atoi(g)
		if err != nil {
			gid = -1
		}
		gids = append(gids, gid)
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
	if os.Getuid() == uid {
		return nil
	}

	// no point continuing if not root
	if !superuser {
		return ErrPermission
	}

	// convert gids...
	var ugids []uint32
	for _, g := range gids {
		if g < 0 {
			continue
		}
		ugids = append(ugids, uint32(g))
	}
	cred := &syscall.Credential{
		Uid:         uint32(uid),
		Gid:         uint32(gid),
		Groups:      ugids,
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
// args with an '=' should be checked and only allowed if there are names?
//
// support glob style wildcards for instance names - allow through, let loopCommand*
// deal with them
//
// process command args in a standard way
// flags will have been handled by another function before this one
// any args with '=' are treated as parameters
//
// a bare argument with a '@' prefix means all instance of type on a remote
//
func parseArgs(cmd Command, rawargs []string) (ct Component, args []string, params []string) {
	var wild bool
	var newnames []string

	if cmd.ParseFlags != nil {
		// loop rawargs and run parseflags every time we see a '-'
		n := 0
		for range rawargs {
			if len(rawargs) < n {
				break
			}
			remains := cmd.ParseFlags(cmd.Name, rawargs[n:])
			rawargs = append(rawargs[:n], remains...)
			n++
		}
	}

	if len(rawargs) == 0 && !cmd.Wildcard {
		return
	}

	logDebug.Println("rawargs, params", rawargs, params)

	// filter in place - pull out all args containing '=' into params
	n := 0
	for _, a := range rawargs {
		if strings.Contains(a, "=") {
			params = append(params, a)
		} else {
			rawargs[n] = a
			n++
		}
	}
	rawargs = rawargs[:n]

	logDebug.Println("rawargs, params", rawargs, params)

	if !cmd.Wildcard {
		if ct = parseComponentName(rawargs[0]); ct == Unknown {
			return
		}
		args = rawargs[1:]
	} else {
		// work through wildcard options
		if len(rawargs) == 0 {
			// no more arguments? wildcard everything
			ct = None
		} else if ct = parseComponentName(rawargs[0]); ct == Unknown {
			// first arg is not a known type, so treat the rest as instance names
			ct = None
			args = rawargs
		} else {
			args = rawargs[1:]
		}

		if cmd.ComponentOnly {
			return
		}

		if len(args) == 0 {
			// no args means all instances
			wild = true
			args = ct.InstanceNames(rALL)
		} else {
			// expand each arg and save results to a new slice
			// if local == "", then all instances on remote (e.g. @remote)
			// if remote == "all" (or none given), then check instance on all remotes
			// @all is not valid - should be no arg
			var nargs []string
			for _, arg := range args {
				// check if not valid first and leave unchanged, skip
				if !(strings.HasPrefix(arg, "@") || validInstanceName(arg)) {
					logDebug.Println("leaving unchanged:", arg)
					nargs = append(nargs, arg)
					continue
				}
				_, local, r := SplitInstanceName(arg, rALL)
				if !r.Loaded() {
					logDebug.Println(arg, "- remote not found")
					// we have tried to match something and it may result in an empty list
					// so don't re-process
					wild = true
					continue
				}

				logDebug.Println("split", arg, "into:", local, r.InstanceName)
				if local == "" {
					// only a '@remote' in arg
					if r.Loaded() {
						rargs := ct.InstanceNames(r)
						nargs = append(nargs, rargs...)
						wild = true
					}
				} else if r == rALL {
					// no '@remote' in arg
					for _, rem := range AllRemotes() {
						logDebug.Printf("checking remote %s for %s", rem.InstanceName, local)
						name := local + "@" + rem.InstanceName
						if ct == None {
							for _, cr := range RealComponents() {
								if i, err := cr.getInstance(name); err == nil && i.Loaded() {
									nargs = append(nargs, name)
									wild = true
								}
							}
						} else if i, err := ct.getInstance(name); err == nil && i.Loaded() {
							nargs = append(nargs, name)
							wild = true
						} else {
							// move the unknown unchanged - file or url - arg so it can later be pushed to params
							// do not set 'wild' though?
							logDebug.Println(arg, "not found")
							nargs = append(nargs, arg)
						}
					}
				} else {
					// save unchanged arg, may be param
					nargs = append(nargs, arg)
					// wild = true
				}
			}
			args = nargs
		}
	}

	logDebug.Println("ct, args, params", ct, args, params)

	m := make(map[string]bool, len(args))
	// traditional loop because we can't modify args in a loop to skip
	for i := 0; i < len(args); i++ {
		name := args[i]
		// filter name here
		if !wild && reservedName(name) {
			logError.Fatalf("%q is reserved name", name)
		}
		// move unknown args to params
		if !validInstanceName(name) {
			params = append(params, name)
			continue
		}
		// ignore duplicates (not params above)
		if m[name] {
			continue
		}
		newnames = append(newnames, name)
		m[name] = true
	}
	args = newnames

	if !cmd.Wildcard {
		return
	}

	// if args is empty, find them all again. ct == None too?
	if len(args) == 0 && Geneos() != "" && !wild {
		args = ct.InstanceNames(rALL)
	}

	logDebug.Println("ct, args, params", ct, args, params)
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
	if GlobalConfig["ReservedNames"] != "" {
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
var validStringRE = regexp.MustCompile(`^\w[:@\.\w -]*$`)

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
func instanceAbsPath(c Instances, file string) (path string) {
	path = filepath.Clean(file)
	if filepath.IsAbs(path) {
		return
	}
	return filepath.Join(c.Home(), path)
}

func deletePaths(c Instances, paths string) (err error) {
	list := filepath.SplitList(paths)
	for _, p := range list {
		// clean path, error on absolute or parent paths, like 'import'
		// walk globbed directories, remove everything
		p, err = cleanRelativePath(p)
		if err != nil {
			return fmt.Errorf("%s %w", p, err)
		}
		// glob here
		m, err := c.Remote().globPath(filepath.Join(c.Home(), p))
		if err != nil {
			return err
		}
		for _, f := range m {
			if err = c.Remote().removeAll(f); err != nil {
				logError.Println(err)
				continue
			}
		}
	}
	return
}

// logdir = LogD relative to Home or absolute
func getLogfilePath(c Instances) (logfile string) {
	logd := filepath.Clean(getString(c, c.Prefix("LogD")))
	switch {
	case logd == "":
		logfile = c.Home()
	case strings.HasPrefix(logd, string(os.PathSeparator)):
		logfile = logd
	default:
		logfile = filepath.Join(c.Home(), logd)
	}
	logfile = filepath.Join(logfile, getString(c, c.Prefix("LogF")))
	return
}

func readSourceBytes(source string) (b []byte) {
	var from io.ReadCloser

	u, err := url.Parse(source)
	if err != nil {
		return
	}

	switch {
	case u.Scheme == "https" || u.Scheme == "http":
		resp, err := http.Get(u.String())
		if err != nil {
			return
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
	if err != nil {
		log.Fatalln(err)
	}
	return
}

//
// load templates from TYPE/templates/[tmpl]* and parse it using the intance data
// write it out to a single file. If tmpl is empty, load all files
//
func createConfigFromTemplate(c Instances, path string, name string, defaultTemplate []byte) (err error) {
	var out io.WriteCloser
	var t *template.Template

	if t, err = template.ParseGlob(c.Remote().GeneosPath(c.Type().String(), "templates", "*")); err != nil {
		// if there are no templates, use internal as a fallback
		log.Printf("No templates found in %s, using internal defaults", c.Remote().GeneosPath(c.Type().String(), "templates"))
		t = template.Must(template.New(name).Parse(string(defaultTemplate)))
	}

	// XXX backup old file

	if out, err = c.Remote().createFile(path, 0660); err != nil {
		log.Printf("Cannot create configurtion file for %s %s", c, path)
		return err
	}
	defer out.Close()

	if err = t.ExecuteTemplate(out, name, c); err != nil {
		log.Println("Cannot create configuration from template(s):", err)
		return err
	}

	return
}

func signalInstance(c Instances, signal syscall.Signal) (err error) {
	pid, err := findInstancePID(c)
	if err != nil {
		return ErrProcNotExist
	}

	if c.Remote() != rLOCAL {
		rem, err := c.Remote().sshOpenRemote()
		if err != nil {
			log.Fatalln(err)
		}
		sess, err := rem.NewSession()
		if err != nil {
			log.Fatalln(err)
		}

		output, err := sess.CombinedOutput(fmt.Sprintf("kill -s %d %d", signal, pid))
		sess.Close()
		if err != nil {
			log.Fatalf("%s FAILED to send %s signal: %s %q", c, signal, err, output)
		}
		logDebug.Printf("%s sent a %s signal", c, signal)
		return nil
	}

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(signal); err != nil && !errors.Is(err, syscall.EEXIST) {
		log.Printf("%s sent a %s signal: %s", c, signal, err)
		return
	}
	logDebug.Printf("%s sent a %s signal", c, signal)
	return
}
