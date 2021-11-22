package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"
)

type Component interface {
	// empty
}

type ComponentType int

const (
	Any ComponentType = iota
	Gateway
	Netprobe
	Licd
	Webserver
)

func (ct ComponentType) String() string {
	switch ct {
	case Any:
		return "any"
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

type Components struct {
	Component
	ITRSHome string
	CompType ComponentType
}

func init() {
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		itrsHome = h
	}
}

func main() {
	var c Component

	if len(os.Args) < 3 {
		log.Fatalln("not enough args")
	}

	switch os.Args[1] {
	case "all", "any":
		//
	case "gateway":
		c = newGateway()
	case "netprobe", "probe":
		c = newNetprobe()
	case "licd":
		c = newLicd()
	case "webserver", "webdashboard", "dashboard":
		log.Println("webserver not supported yet")
		os.Exit(0)
	case "list":
		// list all compoents, annotated
	default:
		log.Println("unknown component", os.Args[1])
		os.Exit(0)
	}

	switch os.Args[2] {
	case "list":
		for _, name := range all(c) {
			fmt.Println(name)
		}
		os.Exit(0)
	case "create":
		// createGateway()
		os.Exit(0)
	}

	if len(os.Args) < 4 {
		log.Fatalln("not enough args and neither list or create")
	}

	var action = os.Args[3]

	names := []string{os.Args[2]}
	if os.Args[2] == "all" {
		names = all(c)
	}

	for _, name := range names {
		switch action {
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
			pid, _, err := getPid(c, name)
			if err != nil {
				log.Println(compType(c), name, "- no valid PID file found")
				continue
			}
			proc, _ := os.FindProcess(pid)
			err = proc.Signal(syscall.Signal(0))
			//
			if err != nil && !errors.Is(err, syscall.EPERM) {
				log.Println(compType(c), name, "process not found", pid)
			} else {
				log.Println(compType(c), name, "running with PID", pid)
			}
		case "refresh":
			// c.refresh(ct, name)
		case "log":

		case "delete":

		default:
			log.Fatalln(compType(c), "unknown action:", action)
		}
	}
}
func start(c Component, name string) {
	cmd, env := loadConfig(c, name)
	if cmd == nil {
		return
	}

	username := getStringField(c, "User")
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
	pid, pidFile, err := getPid(c, name)
	if err != nil {
		//		log.Println("cannot get PID for", name)
		return
	}

	// send sigterm
	log.Println("stopping", compType(c), name, "with PID", pid)

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		log.Println(compType(c), "process not found, removing PID file")
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
			log.Println(compType(c), "terminated")
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

func all(c Component) []string {
	dir := filepath.Join(root(c), compType(c).String()+"s")
	return dirs(dir)
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
		if !fv.CanSet() || fv.Kind() != reflect.String {
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
				fv.SetString(b.String())
			} else {
				fv.SetString(def)
			}
		}

	}
}

func loadConfig(c Component, name string) (cmd *exec.Cmd, env []string) {
	t := compType(c).String()
	prefix := strings.Title(t[0:4])

	wd := filepath.Join(root(c), t+"s", name)
	if err := os.Chdir(wd); err != nil {
		log.Println("cannot chdir() to", wd)
		return
	}
	rcFile, err := os.Open(t + ".rc")
	if err != nil {
		log.Println("cannot open ", t, ".rc")
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
		case prefix + "Opts":
			setStringFieldSlice(c, prefix+"Opts", strings.Fields(v))
		case "BinSuffix":
			setStringField(c, k, v)
		default:
			if strings.HasPrefix(k, prefix) {
				setStringField(c, k, v)
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

	binary := filepath.Join(getStringField(c, "Bins"), getStringField(c, "Base"), getStringField(c, "BinSuffix"))
	logFile := filepath.Join(getStringField(c, "LogD"), name, getStringField(c, "LogF"))

	env = append(env, "LD_LIBRARY_PATH="+getStringField(c, "Libs"))
	args := []string{name, "-logfile", logFile}
	args = append(args, getStringFieldSlice(c, "Opts")...)
	cmd = exec.Command(binary, args...)

	return
}

func run(c Component, name string, cmd *exec.Cmd, env []string) {
	wd := filepath.Join(root(c), compType(c).String()+"s", name)

	// actually run the process
	cmd.Env = append(os.Environ(), env...)
	err := cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	if cmd.Process != nil {
		// write pid file
		pidFile := filepath.Join(wd, compType(c).String()+".pid")
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

func root(c Component) string {
	return getStringField(c, "Root")
}

func compType(c Component) ComponentType {
	v := reflect.ValueOf(c).Elem().FieldByName("CompType")
	if v.IsValid() {
		return v.Interface().(ComponentType)
	}
	return Any
}

func getStringField(c Component, name string) string {
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

func setStringField(c Component, k, v string) {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}
	fv = fv.FieldByName(k)
	if fv.IsValid() && fv.CanSet() {
		fv.SetString(v)
	}
	//return r
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
