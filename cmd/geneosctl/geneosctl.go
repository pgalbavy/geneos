package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Component interface {
	// empty
}

type ComponentType int

const (
	None ComponentType = iota
	Gateway
	Netprobe
	Licd
	Webserver
)

func (ct ComponentType) String() string {
	switch ct {
	case None:
		return "none"
	case Gateway:
		return "gateway"
	case Netprobe:
		return "netprobe"
	case Licd:
		return "licd"
	case Webserver:
		return "webserver"
	}
	return "unknown"
}

func ct(component string) ComponentType {
	switch strings.ToLower(component) {
	case "gateway":
		return Gateway
	case "netprobe", "probe":
		return Netprobe
	case "licd":
		return Licd
	case "webserver", "webdashboard":
		return Webserver
	default:
		return None
	}
}

type Components struct {
	Component `json:"-"`
	ITRSHome  string        `json:"-"`
	CompType  ComponentType `json:"-"`
}

func init() {
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		itrsHome = h
	}
}

//
// redo
//
// geneosctl COMMAND [COMPONENT] [NAME]
//
// COMPONENT = "" | gateway | netprobe | licd | webserver
// COMMAND = start | stop | restart | status | command | ...
//
func main() {
	if len(os.Args) < 2 {
		log.Fatalln("[usage here]: not enough args")
	}

	var command = strings.ToLower(os.Args[1])

	var comp ComponentType
	var names []string

	if len(os.Args) > 2 {
		comp = ct(os.Args[2])
		if comp == None {
			// this may be a name instead
			names = os.Args[2:]
		} else {
			names = os.Args[3:]
		}
	}

	if len(names) == 0 {
		// no names, check special commands and exit
		switch command {
		case "list":
		case "version":
		case "help":
		case "status":
			names = compAll(comp)
		default:
			os.Exit(0)
		}
	}

	// loop over names, if any supplied
	for _, name := range names {
		var c Component

		switch comp {
		case Gateway:
			c = newGateway(name)
		case Netprobe:
			c = newNetprobe(name)
		case Licd:
			c = newLicd(name)
		case Webserver:
			log.Println("webserver not supported yet")
			os.Exit(0)
		default:
			log.Println("unknown component", comp)
			os.Exit(0)
		}

		switch command {
		case "create":
			// create one or more components, type is option
			// if no type create multiple different components for each names
		case "start":
			start(c, name)
		case "stop":
			stop(c, name)
		case "restart":
			stop(c, name)
			start(c, name)
		case "command":
			cmd, env := loadConfig(c, name)
			if cmd != nil {
				log.Printf("command: %q\n", cmd.String())
				log.Println("extra environment:")
				for _, e := range env {
					log.Println(e)
				}
			}
			log.Println("end")
		case "details":
			//
		case "status":
			pid, err := findProc(c, name) // getPid(c, name)
			if err != nil {
				log.Println(compType(c), name, ":", err)
				continue
			}
			log.Println(compType(c), name, "running with PID", pid)

		case "refresh":
			// c.refresh(ct, name)
		case "log":
		case "delete":
		default:
			log.Fatalln(compType(c), "[usage here] unknown command:", command)
		}
	}
}

func start(c Component, name string) {
	cmd, env := loadConfig(c, name)
	if cmd == nil {
		return
	}

	username := getStringWithPrefix(c, "User")
	if len(username) != 0 {
		u, _ := user.Current()
		if username != u.Username {
			log.Println("can't change user to", username)
			return
		}
	}

	run(c, name, cmd, env)
}

func stop(c Component, name string) {
	pid, err := findProc(c, name)
	if err != nil {
		//		log.Println("cannot get PID for", name)
		return
	}

	// send sigterm
	log.Println("stopping", compType(c), name, "with PID", pid)

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		log.Println(compType(c), "process not found")
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
			log.Println(compType(c), "terminated")
			return
		}
	}
	// sigkill
	if err = proc.Signal(syscall.SIGKILL); err != nil {
		log.Println("sending SIGKILL failed:", err)
		return
	}
}

func dirs(dir string) []string {
	files, _ := os.ReadDir(dir)
	components := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			components = append(components, file.Name())
		}
	}
	return components
}

func compAll(comp ComponentType) []string {
	return dirs(compRootDir(comp))
}

var funcs = template.FuncMap{"join": filepath.Join}

func newComponent(c interface{}) {
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
			if strings.Contains(def, "{{") {
				val, err := template.New(ft.Name).Funcs(funcs).Parse(def)
				if err != nil {
					log.Println("parse error:", def)
					continue
				}

				var b bytes.Buffer
				err = val.Execute(&b, c)
				if err != nil {
					log.Println("cannot convert:", def)
				}
				setField(c, ft.Name, b.String())
			} else {
				setField(c, ft.Name, def)
			}
		}

	}
}

func loadConfig(c Component, name string) (cmd *exec.Cmd, env []string) {
	t := compType(c).String()
	prefix := strings.Title(t[0:4])

	wd := filepath.Join(compRootDir(compType(c)), name)
	if err := os.Chdir(wd); err != nil {
		log.Println("cannot chdir() to", wd)
		return
	}
	jsonFile, err := os.ReadFile(t + ".json")
	if err == nil {
		json.Unmarshal(jsonFile, &c)
	} else {
		// load an rc file and try to write out the JSON version
		rcFile, err := os.Open(t + ".rc")
		if err != nil {
			log.Println("cannot open ", t, ".rc")
			return
		}
		defer rcFile.Close()

		log.Printf("loading config from %s/%s.rc", wd, t)

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

		// log.Printf("defaults: %+v\n", c)
		for k, v := range confs {
			switch k {
			case prefix + "Opts":
				setStringFieldSlice(c, prefix+"Opts", strings.Fields(v))
			case "BinSuffix":
				setField(c, k, v)
			default:
				if strings.HasPrefix(k, prefix) {
					setField(c, k, v)
				} else {
					// set env var
					env = append(env, fmt.Sprintf("%s=%s", k, v))
				}
			}
		}

		j, err := json.MarshalIndent(c, "", "    ")
		if err != nil {
			log.Println("json marshal failed:", err)
		} else {
			log.Printf("%s\n", string(j))
			err = os.WriteFile(t+".json", j, 0666)
			if err != nil {
				log.Println("cannot write JSON config file:", err)
			}
		}
	}

	// build command line and env vars
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "/bin/bash"
	}

	// XXX abstract this stuff away
	binary := filepath.Join(getStringWithPrefix(c, "Bins"), getStringWithPrefix(c, "Base"), getString(c, "BinSuffix"))
	resourcesDir := filepath.Join(getStringWithPrefix(c, "Bins"), getStringWithPrefix(c, "Base"), "resources")

	logFile := filepath.Join(getStringWithPrefix(c, "LogD"), name, getStringWithPrefix(c, "LogF"))
	setupFile := filepath.Join(getStringWithPrefix(c, "Home"), "gateway.setup.xml")

	// XXX find common envs - JAVA_HOME etc.
	env = append(env, "LD_LIBRARY_PATH="+getStringWithPrefix(c, "Libs"))

	var args []string
	// XXX args and env vary depending on Component type - the below is for Gateway
	switch compType(c) {
	case Gateway:
		args = []string{
			/* "-gateway-name",  */ name,
			"-setup-file", setupFile,
			"-resources-dir", resourcesDir,
			"-log", logFile,
			"-licd-host", getStringWithPrefix(c, "LicH"),
			"-licd-port", getIntWithPrefix(c, "LicP"),
			// "-port", getIntWithPrefix(c, "Port"),
		}
	case Netprobe:
		env = append(env, "LOGFILE="+logFile)
	default:
	}

	args = append(args, getStringFieldSlice(c, "Opts")...)
	cmd = exec.Command(binary, args...)

	return
}

func run(c Component, name string, cmd *exec.Cmd, env []string) {
	// wd := filepath.Join(compRootDir(compType(c)), name)

	// actually run the process
	cmd.Dir = getStringWithPrefix(c, "Home")
	cmd.Env = append(os.Environ(), env...)
	errfile := filepath.Join(getStringWithPrefix(c, "LogD"), name, compType(c).String()+".txt")

	out, err := os.OpenFile(errfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("cannot open output file")
	}
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("process", cmd.Process.Pid)

	if cmd.Process != nil {
		// write pid file
		/* pidFile := filepath.Join(wd, compType(c).String()+".pid")
		if file, err := os.Create(pidFile); err != nil {
			log.Printf("cannot open %q for writing", pidFile)
		} else {
			fmt.Fprintln(file, cmd.Process.Pid)
			file.Close()
		} */
		// detach
		cmd.Process.Release()
	}
}

func compRootDir(comp ComponentType) string {
	return filepath.Join(itrsHome, comp.String(), comp.String()+"s")
}

func compType(c Component) ComponentType {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}
	if !fv.IsValid() {
		return None
	}
	v := fv.FieldByName("CompType")

	if v.IsValid() {
		return v.Interface().(ComponentType)
	}
	return None
}

func getString(c Component, name string) string {
	v := reflect.ValueOf(c).Elem().FieldByName(name)
	if v.IsValid() && v.Kind() == reflect.String {
		return v.String()
	}
	return ""
}

func getIntWithPrefix(c Component, name string) string {
	t := compType(c).String()
	prefix := strings.Title(t[0:4])

	v := reflect.ValueOf(c).Elem().FieldByName(prefix + name)
	if v.IsValid() && v.Kind() == reflect.Int {
		return fmt.Sprintf("%v", v.Int())
	}
	return ""
}

func getStringWithPrefix(c Component, name string) string {
	t := compType(c).String()
	prefix := strings.Title(t[0:4])

	v := reflect.ValueOf(c).Elem().FieldByName(prefix + name)
	if v.IsValid() && v.Kind() == reflect.String {
		return v.String()
	}
	return ""
}

func getStringFieldSlice(c Component, names ...string) (fields []string) {
	t := compType(c).String()
	prefix := strings.Title(t[0:4])

	v := reflect.ValueOf(c).Elem()

	fv := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf("abc")), 0, 5)

	for _, name := range names {
		f := v.FieldByName(prefix + name)
		if f.IsValid() {
			switch f.Kind() {
			case reflect.String:
				fv = reflect.Append(fv, f)
				// fields = append(fields, f.String())
			case reflect.Slice:
				fv = reflect.AppendSlice(fv, f)
			}
		}
	}
	fields = fv.Interface().([]string)
	return
}

func setField(c Component, k string, v string) {
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

func setStringFieldSlice(c Component, k string, v []string) {
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
