package instance

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/template"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/pkg/logger"
)

// The Instance type is the common data shared by all instance / component types
type Instance struct {
	geneos.Instance `json:"-"`
	L               *sync.RWMutex     `json:"-"`
	Conf            *viper.Viper      `json:"-"`
	InstanceHost    *host.Host        `json:"-"`
	Component       *geneos.Component `json:"-"`
	ConfigLoaded    bool              `json:"-"`
	Env             []string          `json:",omitempty"`
}

var (
	log      = logger.Log
	logDebug = logger.Debug
	logError = logger.Error
)

// locate a process instance
//
// the component type must be part of the basename of the executable and
// the component name must be on the command line as an exact and
// standalone args
//
// walk the /proc directory (local or remote) and find the matching pid
// this is subject to races, but not much we can do
func GetPID(c geneos.Instance) (pid int, err error) {
	var pids []int
	binsuffix := c.V().GetString("BinSuffix")

	// safe to ignore error as it can only be bad pattern,
	// which means no matches to range over
	dirs, _ := c.Host().Glob("/proc/[0-9]*")

	for _, dir := range dirs {
		p, _ := strconv.Atoi(filepath.Base(dir))
		pids = append(pids, p)
	}

	sort.Ints(pids)

	var data []byte
	for _, pid = range pids {
		if data, err = c.Host().ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err != nil {
			// process may disappear by this point, ignore error
			continue
		}
		args := bytes.Split(data, []byte("\000"))
		execfile := filepath.Base(string(args[0]))
		switch c.Type() {
		case geneos.ParseComponentName("webserver"):
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
	return 0, os.ErrProcessDone
}

func GetPIDInfo(c geneos.Instance) (pid int, uid uint32, gid uint32, mtime int64, err error) {
	pid, err = GetPID(c)
	if err == nil {
		var s host.FileStat
		s, err = c.Host().Stat(fmt.Sprintf("/proc/%d", pid))
		return pid, s.Uid, s.Gid, s.Mtime, err
	}
	return 0, 0, 0, 0, os.ErrProcessDone
}

// separate reserved words and invalid syntax
//
func ReservedName(in string) (ok bool) {
	logDebug.Printf("checking %q", in)
	if geneos.ParseComponentName(in) != nil {
		logDebug.Println("matches a reserved word")
		return true
	}
	if viper.GetString("reservednames") != "" {
		list := strings.Split(in, ",")
		for _, n := range list {
			if strings.EqualFold(in, strings.TrimSpace(n)) {
				logDebug.Println("matches a user defined reserved name")
				return true
			}
		}
	}
	return
}

// spaces are valid - dumb, but valid - for now
// if the name starts with  number then the next character cannot be a number or '.'
// to help distinguish from versions
var validStringRE = regexp.MustCompile(`^\w[\w-]+[:@\.\w -]*$`)

// return true while a string is considered a valid instance name
//
// used to consume instance names until parameters are then passed down
//
func ValidInstanceName(in string) (ok bool) {
	ok = validStringRE.MatchString(in)
	if !ok {
		logDebug.Println("no rexexp match:", in)
	}
	return
}

// given a filename or path, prepend the instance home directory
// if not absolute, and clean
func Abs(c geneos.Instance, file string) (path string) {
	path = filepath.Clean(file)
	if filepath.IsAbs(path) {
		return
	}
	return filepath.Join(c.Home(), path)
}

// given a pathlist (typically ':') seperated list of paths, remove all
// files and directories
func RemovePaths(c geneos.Instance, paths string) (err error) {
	list := filepath.SplitList(paths)
	for _, p := range list {
		// clean path, error on absolute or parent paths, like 'import'
		// walk globbed directories, remove everything
		p, err = host.CleanRelativePath(p)
		if err != nil {
			return fmt.Errorf("%s %w", p, err)
		}
		// glob here
		m, err := c.Host().Glob(filepath.Join(c.Home(), p))
		if err != nil {
			return err
		}
		for _, f := range m {
			if err = c.Host().RemoveAll(f); err != nil {
				logError.Println(err)
				continue
			}
		}
	}
	return
}

// logdir = LogD relative to Home or absolute
func LogFile(c geneos.Instance) (logfile string) {
	logd := filepath.Clean(c.V().GetString(c.Prefix("logd")))
	switch {
	case logd == "":
		logfile = c.Home()
	case filepath.IsAbs(logd):
		logfile = logd
	default:
		logfile = filepath.Join(c.Home(), logd)
	}
	logfile = filepath.Join(logfile, c.V().GetString(c.Prefix("logf")))
	return
}

func Signal(c geneos.Instance, signal syscall.Signal) (err error) {
	pid, err := GetPID(c)
	if err != nil {
		return os.ErrProcessDone
	}

	if c.Host() == host.LOCAL {
		proc, _ := os.FindProcess(pid)
		if err = proc.Signal(signal); err != nil && !errors.Is(err, syscall.EEXIST) {
			log.Printf("%s FAILED to send a signal %d: %s", c, signal, err)
			return
		}
		logDebug.Printf("%s sent a signal %d", c, signal)
		return nil
	}

	rem, err := c.Host().Dial()
	if err != nil {
		logError.Fatalln(err)
	}
	sess, err := rem.NewSession()
	if err != nil {
		logError.Fatalln(err)
	}

	output, err := sess.CombinedOutput(fmt.Sprintf("kill -s %d %d", signal, pid))
	sess.Close()
	if err != nil {
		log.Printf("%s FAILED to send signal %d: %s %q", c, signal, err, output)
		return
	}
	logDebug.Printf("%s sent a signal %d", c, signal)
	return nil
}

// return a slice instances for a given component type
func GetAll(r *host.Host, ct *geneos.Component) (confs []geneos.Instance) {
	if ct == nil {
		for _, c := range geneos.RealComponents() {
			confs = append(confs, GetAll(r, c)...)
		}
		return
	}
	for _, name := range AllNames(r, ct) {
		i, err := Get(ct, name)
		if err != nil {
			continue
		}
		confs = append(confs, i)
	}

	return
}

// return an instance of component ct. loads the config.
func Get(ct *geneos.Component, name string) (c geneos.Instance, err error) {
	if ct == nil {
		return nil, geneos.ErrInvalidArgs
	}

	c = ct.New(name)
	if c == nil {
		return nil, geneos.ErrInvalidArgs
	}
	err = c.Load()
	return
}

// construct and return a slice of a/all component types that have
// a matching name
func MatchAll(ct *geneos.Component, name string) (c []geneos.Instance) {
	_, local, r := SplitName(name, host.ALL)
	if !r.Loaded() {
		logDebug.Println("remote", r, "not loaded")
		return
	}

	if ct == nil {
		for _, t := range geneos.RealComponents() {
			c = append(c, MatchAll(t, name)...)
		}
		return
	}

	for _, name := range AllNames(r, ct) {
		logDebug.Println("ct, name =", ct, name)

		// for case insensitive match change to EqualFold here
		_, ldir, _ := SplitName(name, host.ALL)
		if filepath.Base(ldir) == local {
			i, err := Get(ct, name)
			if err != nil {
				logError.Println(err)
				continue
			}
			c = append(c, i)
		}
	}

	return
}

// Looks for exactly one matching instance across types and remotes
// returns Invalid Args if zero of more than 1 match
func Match(ct *geneos.Component, name string) (c geneos.Instance, err error) {
	list := MatchAll(ct, name)
	if len(list) == 0 {
		err = os.ErrNotExist
		return
	}
	if len(list) == 1 {
		c = list[0]
		return
	}
	err = geneos.ErrInvalidArgs
	return
}

// get all used ports in config files on a specific remote
// this will not work for ports assigned in component config
// files, such as gateway setup or netprobe collection agent
//
// returns a map
func GetPorts(r *host.Host) (ports map[int]*geneos.Component) {
	if r == host.ALL {
		logError.Fatalln("getports() call with all remotes")
	}
	ports = make(map[int]*geneos.Component)
	for _, c := range GetAll(r, nil) {
		if !c.Loaded() {
			log.Println("cannot load configuration for", c)
			continue
		}
		if port := c.V().GetInt(c.Prefix("Port")); port != 0 {
			ports[int(port)] = c.Type()
		}
	}
	return
}

// syntax of ranges of ints:
// x,y,a-b,c..d m n o-p
// also open ended A,N-,B
// command or space seperated?
// - or .. = inclusive range
//
// how to represent
// split, for range, check min-max -> max > min
// repeats ignored
// special ports? - nah
//

// given a range, find the first unused port
//
// range is comma or two-dot seperated list of
// single number, e.g. "7036"
// min-max inclusive range, e.g. "7036-8036"
// start- open ended range, e.g. "7041-"
//
// some limits based on https://en.wikipedia.org/wiki/List_of_TCP_and_UDP_port_numbers
//
// not concurrency safe at this time
//
func NextPort(r *host.Host, ct *geneos.Component) int {
	from := viper.GetString(ct.PortRange)
	used := GetPorts(r)
	ps := strings.Split(from, ",")
	for _, p := range ps {
		// split on comma or ".."
		m := strings.SplitN(p, "-", 2)
		if len(m) == 1 {
			m = strings.SplitN(p, "..", 2)
		}

		if len(m) > 1 {
			min, err := strconv.Atoi(m[0])
			if err != nil {
				continue
			}
			if m[1] == "" {
				m[1] = "49151"
			}
			max, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			if min >= max {
				continue
			}
			for i := min; i <= max; i++ {
				if _, ok := used[i]; !ok {
					// found an unused port
					return i
				}
			}
		} else {
			p1, err := strconv.Atoi(m[0])
			if err != nil || p1 < 1 || p1 > 49151 {
				continue
			}
			if _, ok := used[p1]; !ok {
				return p1
			}
		}
	}
	return 0
}

//
// return the base package name and the version it links to.
// if not a link, then return the same
// follow a limited number of links (10?)
func Version(c geneos.Instance) (base string, underlying string, err error) {
	basedir := c.V().GetString(c.Prefix("Bins"))
	base = c.V().GetString(c.Prefix("Base"))
	underlying = base
	for {
		basepath := filepath.Join(basedir, underlying)
		var st host.FileStat
		st, err = c.Host().Lstat(basepath)
		if err != nil {
			underlying = "unknown"
			return
		}
		if st.St.Mode()&fs.ModeSymlink != 0 {
			underlying, err = c.Host().ReadLink(basepath)
			if err != nil {
				underlying = "unknown"
				return
			}
		} else {
			break
		}
	}
	return
}

// given a component type and a slice of args, call the function for each arg
//
// try to use go routines here - mutexes required
func ForAll(ct *geneos.Component, fn func(geneos.Instance, []string) error, args []string, params []string) (err error) {
	n := 0
	logDebug.Println("args, params", args, params)
	for _, name := range args {
		cs := MatchAll(ct, name)
		if len(cs) == 0 {
			log.Println("no matches for", ct, name)
			continue
			// return os.ErrNotExist
		}
		n++
		for _, c := range cs {
			if err = fn(c, params); err != nil && !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, geneos.ErrNotSupported) {
				log.Println(c, err)
			}
		}
	}
	if n == 0 {
		return os.ErrNotExist
	}
	return nil
}

// Return a slice of all instanceNames for a given geneos. No
// checking is done to validate that the directory is a populated
// instance.
//
func AllNames(r *host.Host, ct *geneos.Component) (names []string) {
	var files []fs.DirEntry

	logDebug.Println("host, ct:", r, ct)
	if r == host.ALL {
		for _, r := range host.AllHosts() {
			names = append(names, AllNames(r, ct)...)
		}
		logDebug.Println("names:", names)
		return
	}

	if ct == nil {
		for _, t := range geneos.RealComponents() {
			// ignore errors, we only care about any files found
			d, _ := r.ReadDir(t.ComponentDir(r))
			files = append(files, d...)
		}
	} else {
		// ignore errors, we only care about any files found
		files, _ = r.ReadDir(ct.ComponentDir(r))
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})
	for i, file := range files {
		// skip for values with the same name as previous
		if i > 0 && i < len(files) && file.Name() == files[i-1].Name() {
			continue
		}
		if file.IsDir() {
			names = append(names, file.Name()+"@"+r.String())
		}
	}
	return
}

// given an instance name in the format [TYPE:]NAME[@HOST] and a default
// host, return a *geneos.Component for the TYPE if given, a string
// for the NAME and a *host.Host - the latter being either from the name
// or the default provided
func SplitName(in string, defaultRemote *host.Host) (ct *geneos.Component, name string, r *host.Host) {
	r = defaultRemote
	parts := strings.SplitN(in, "@", 2)
	name = parts[0]
	if len(parts) > 1 {
		r = host.Get(host.Name(parts[1]))
	}
	parts = strings.SplitN(name, ":", 2)
	if len(parts) > 1 {
		ct = geneos.ParseComponentName(parts[0])
		name = parts[1]
	}
	return
}

// buildCmd gathers the path to the binary, arguments and any environment variables
// for an instance and returns an exec.Cmd, almost ready for execution. Callers
// will add more details such as working directories, user and group etc.
func BuildCmd(c geneos.Instance) (cmd *exec.Cmd, env []string) {
	binary := c.V().GetString(c.Prefix("Exec"))

	args, env := c.Command()

	opts := strings.Fields(c.V().GetString(c.Prefix("Opts")))
	args = append(args, opts...)
	// XXX find common envs - JAVA_HOME etc.
	env = append(env, c.V().GetStringSlice("Env")...)
	env = append(env, "LD_LIBRARY_PATH="+c.V().GetString(c.Prefix("Libs")))
	cmd = exec.Command(binary, args...)

	return
}

// a template function to support "{{join .X .Y}}"
var textJoinFuncs = template.FuncMap{"join": filepath.Join}

// SetDefaults() is a common function called by component factory
// functions to iterate over the component specific instance
// struct and set the defaults as defined in the 'defaults'
// struct tags.
func SetDefaults(c geneos.Instance, name string) (err error) {
	c.V().SetDefault("name", name)
	if c.Type().Defaults != nil {
		// set bootstrap values used by templates
		c.V().Set("remoteroot", c.Host().V().GetString("geneos"))
		for _, s := range c.Type().Defaults {
			p := strings.SplitN(s, "=", 2)
			k, v := p[0], p[1]
			val, err := template.New(k).Funcs(textJoinFuncs).Parse(v)
			if err != nil {
				log.Println(c, "setDefaults parse error:", v)
				return err
			}
			var b bytes.Buffer
			if c.V() == nil {
				logError.Println("no viper found")
			}
			if err = val.Execute(&b, c.V().AllSettings()); err != nil {
				log.Println(c, "cannot set defaults:", v)
				return err
			}
			c.V().SetDefault(k, b.String())
		}
		// remove these so they don't pollute written out files
		c.V().Set("remoteroot", nil)
	}

	return
}

func IsDisabled(c geneos.Instance) bool {
	d := ConfigPathWithExt(c, geneos.DisableExtension)
	if f, err := c.Host().Stat(d); err == nil && f.St.Mode().IsRegular() {
		return true
	}
	return false
}
