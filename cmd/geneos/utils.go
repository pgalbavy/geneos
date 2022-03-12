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
	dirs, _ := globPath(c.Location(), "/proc/[0-9]*")

	for _, dir := range dirs {
		p, _ := strconv.Atoi(filepath.Base(dir))
		pids = append(pids, p)
	}

	sort.Ints(pids)

	binsuffix := getString(c, "BinSuffix")

	for _, pid = range pids {
		var data []byte
		data, err = readFile(c.Location(), fmt.Sprintf("/proc/%d/cmdline", pid))
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
		s, err = statFile(c.Location(), fmt.Sprintf("/proc/%d", pid))
		return pid, s.uid, s.uid, s.mtime, err
	}
	return 0, 0, 0, 0, ErrProcNotExist
}

func getUser(username string) (uid, gid uint32, gids []uint32, err error) {
	uid, gid = math.MaxUint32, math.MaxUint32

	if username == "" {
		username = GlobalConfig["DefaultUser"]
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
func defaultArgs(rawargs []string) (ct Component, args []string, params []string) {
	var wild bool
	// work through wildcard options
	if len(rawargs) == 0 {
		// no more arguments? wildcard everything
		ct = None
	} else if ct = parseComponentName(rawargs[0]); ct == Unknown {
		// first arg is not a known type, so treat the rest as instance names
		ct = None
		args = rawargs
		if len(args) == 0 {
		}
	} else {
		args = rawargs[1:]
	}

	if len(args) == 0 {
		wild = true
		args = ct.instanceNames(ALL)
	}

	// check args for prefix '@', expand
	//

	logDebug.Println("ct, args, params", ct, args, params)

	// make sure names/args are unique but retain order
	// check for reserved names here?
	// do space exchange inbound here
	var newnames []string

	m := make(map[string]bool, len(args))
	for i, name := range args {
		// filter name here - only if not wildcarded though, as we get those from directory names
		if !wild && reservedName(name) {
			logError.Fatalf("%q is reserved instance name", name)
		}
		if !validInstanceName(name) {
			// first invalid name end processing, save the rest as params
			logDebug.Printf("%q is not a valid instance name, stopped processing args", name)
			params = args[i:]
			break
		}
		// simply ignore duplicates
		if m[name] {
			continue
		}
		newnames = append(newnames, name)
		m[name] = true
	}
	args = newnames

	// repeat if args is now empty (all params)
	if len(args) == 0 {
		wild = true
		args = ct.instanceNames(ALL) //  allArgsForComponent()
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
	for i, name := range args {
		// filter name here
		if reservedName(name) {
			logError.Fatalf("%q is reserved instance name", name)
		}
		if !validInstanceName(name) {
			// first invalid name end processing, save the rest as params
			logDebug.Printf("%q is not a valid instance name, stopped processing args", name)
			params = args[i:]
			break
		}
		// simply ignore duplicates
		if m[name] {
			continue
		}
		newnames = append(newnames, name)
		m[name] = true
	}
	args = newnames

	logDebug.Println("params:", params)
	return
}

// no wildcards or parameters, just check the first arg for a valid component type and
// return the rest as args. the caller has to check component type for validity
func checkComponentArg(rawargs []string) (ct Component, args []string, params []string) {
	if len(rawargs) == 0 {
		ct = None
	} else if ct = parseComponentName(rawargs[0]); ct == Unknown {
		ct = None
		args = rawargs
	} else {
		args = rawargs[1:]
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

func readSourceString(source string) (s string) {
	b := readSourceBytes(source)
	return string(b)
}

func writeTemplate(c Instances, path string, tmpl string) (err error) {
	var out io.WriteCloser

	// default config XML etc.
	t, err := template.New("empty").Funcs(textJoinFuncs).Parse(tmpl)
	if err != nil {
		logError.Fatalln(err)
	}

	out, err = createFile(c.Location(), path, 0660)
	if err != nil {
		log.Fatalln(err)
	}
	defer out.Close()

	if err = t.Execute(out, c); err != nil {
		logError.Fatalln(err)
	}

	return
}

func signalInstance(c Instances, signal syscall.Signal) (err error) {
	pid, err := findInstancePID(c)
	if err != nil {
		return ErrProcNotExist
	}

	if c.Location() != LOCAL {
		rem, err := sshOpenRemote(c.Location())
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
			log.Fatalf("%s %s@%s FAILED to send %s signal: %s %q", c.Type(), c.Name(), c.Location(), signal, err, output)
		}
		logDebug.Printf("%s %s@%s sent a %s signal", c.Type(), c.Name(), c.Location(), signal)
		return nil
	}

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(signal); err != nil && !errors.Is(err, syscall.EEXIST) {
		log.Printf("%s %s@%s sent a %s signal: %s", c.Type(), c.Name(), c.Location(), signal, err)
		return
	}
	logDebug.Printf("%s %s@%s sent a %s signal", c.Type(), c.Name(), c.Location(), signal)
	return
}
