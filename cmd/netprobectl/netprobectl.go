package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
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

type netprobeSettings struct {
	ITRSHome  string
	NetpRoot  string   `default:"{{join .ITRSHome \"netprobe\"}}"`
	NetpBins  string   `default:"{{join .ITRSHome \"packages\" \"netprobe\"}}"`
	NetpBase  string   `default:"active_prod"`
	NetpLogD  string   `default:"{{join .NetpRoot \"netprobes\"}}"`
	NetpLogF  string   `default:"netprobe.log"`
	NetpMode  string   `default:"background"`
	NetpOpts  []string // =-nopassword
	NetpLibs  string   `default:"{{join .NetpBins .NetpBase \"lib64\"}}:{{join .NetpBins .NetpBase}}"`
	NetpUser  string
	BinSuffix string `default:"netprobe.linux_64"`
}

var itrsHome string = "/opt/itrs"

func init() {
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		itrsHome = h
	}
}

func main() {
	npSettings := defaultSettings()

	if len(os.Args) < 2 {
		log.Fatalln("not enough args")
	}
	switch os.Args[1] {
	case "list":
		listProbes(npSettings)
		os.Exit(0)
	case "create":
		// createGateway()
		os.Exit(0)
	}

	if len(os.Args) < 3 {
		log.Fatalln("not enough args and lot list or create")
	}

	var probeName = os.Args[1]
	var action = os.Args[2]

	probes := []string{probeName}
	if probeName == "all" {
		probes = allProbes(npSettings)
	}

	for _, probe := range probes {
		switch action {
		case "start":
			startProbe(npSettings, probe)
		case "stop":
			stopProbe(npSettings, probe)
		case "restart":
			stopProbe(npSettings, probe)
			startProbe(npSettings, probe)
		case "command":
			cmd, env := setupProbe(npSettings, probe)
			if cmd != nil {
				log.Printf("command: %q\n", cmd.String())
				log.Println("extra environment:")
				for _, e := range env {
					log.Println(e)
				}
			}
		case "details":
		case "status":
			pid, _, err := getPid(npSettings, probe)
			if err != nil {
				log.Println("netprobe", probe, "- no valid PID file found")
				continue
			}
			proc, _ := os.FindProcess(pid)
			err = proc.Signal(syscall.Signal(0))
			//
			if err != nil && !errors.Is(err, syscall.EPERM) {
				log.Println("netprobe", probe, "process not found")
			} else {
				log.Println("netprobe", probe, "running with PID", pid)
			}
		case "refresh":
			// send a SIGUSR1
			pid, _, err := getPid(npSettings, probe)
			if err != nil {
				continue
			}
			proc, _ := os.FindProcess(pid)
			if err = proc.Signal(syscall.SIGUSR1); err != nil {
				log.Println("netprobe", probe, "reload config failed")
				continue
			}
		case "log":

		case "delete":

		default:
			log.Fatalln("unknown action:", action)
		}
	}
}

func listProbes(settings netprobeSettings) {
	probes := allProbes(settings)
	for _, probe := range probes {
		fmt.Println(probe)
	}
}

func allProbes(settings netprobeSettings) []string {
	probesDir := filepath.Join(settings.NetpRoot, "netprobes")
	files, _ := os.ReadDir(probesDir)
	netprobes := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			netprobes = append(netprobes, file.Name())
		}
	}
	return netprobes
}

func startProbe(settings netprobeSettings, name string) {
	cmd, env := setupProbe(settings, name)
	if cmd == nil {
		return
	}

	if len(settings.NetpUser) != 0 {
		u, _ := user.Current()
		if settings.NetpUser != u.Username {
			log.Println("can't change user to", settings.NetpUser)
			return
		}
	}

	runNetprobe(settings, name, cmd, env)
}

func setupProbe(settings netprobeSettings, name string) (cmd *exec.Cmd, env []string) {
	probeDir := filepath.Join(settings.NetpRoot, "netprobes", name)
	if err := os.Chdir(probeDir); err != nil {
		log.Println("cannot chdir() to", probeDir)
		return
	}
	rcFile, err := os.Open("netprobe.rc")
	if err != nil {
		log.Println("cannot open netprobe.rc")
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
		case "NetpOpts":
			settings.NetpOpts = strings.Fields(v)
		case "BinSuffix":
			settings.setField(k, v)
		default:
			if strings.HasPrefix(k, "Netp") {
				settings.setField(k, v)
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

	binary := filepath.Join(settings.NetpBins, settings.NetpBase, settings.BinSuffix)
	logFile := filepath.Join(settings.NetpLogD, name, settings.NetpLogF)

	env = append(env, "LD_LIBRARY_PATH="+settings.NetpLibs)
	args := []string{name, "-logfile", logFile}
	args = append(args, settings.NetpOpts...)
	cmd = exec.Command(binary, args...)

	return
}

func runNetprobe(settings netprobeSettings, name string, cmd *exec.Cmd, env []string) {
	probeHome := filepath.Join(settings.NetpRoot, "netprobes", name)

	// actually run the process
	cmd.Env = append(os.Environ(), env...)
	err := cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	if cmd.Process != nil {
		// write pid file
		pidFile := filepath.Join(probeHome, "netprobe.pid")
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

func getPid(settings netprobeSettings, name string) (pid int, pidFile string, err error) {
	probeHome := filepath.Join(settings.NetpRoot, "netprobes", name)
	// open pid file
	pidFile = filepath.Join(probeHome, "netprobe.pid")
	pidBytes, err := ioutil.ReadFile(pidFile)
	if err != nil {
		err = fmt.Errorf("cannot read PID file")
		return
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		err = fmt.Errorf("cannot convert PID to int:", err)
		return
	}
	return
}

func stopProbe(settings netprobeSettings, name string) {
	pid, pidFile, err := getPid(settings, name)
	if err != nil {
		//		log.Println("cannot get PID for", name)
		return
	}

	// send sigterm
	log.Println("stopping netprobe", name, "with PID", pid)

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		log.Println("netprobe process not found, removing PID file")
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
			log.Println("netprobe terminated")
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

func defaultSettings() netprobeSettings {
	settings := &netprobeSettings{}
	// Bootstrap
	settings.ITRSHome = itrsHome
	// empty slice
	settings.NetpOpts = []string{}

	st := reflect.TypeOf(*settings)
	sv := reflect.ValueOf(settings).Elem()
	funcs := template.FuncMap{"join": filepath.Join}

	for i := 0; i < st.NumField(); i++ {
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
				err = val.Execute(&b, settings)
				fv.SetString(b.String())
			} else {
				fv.SetString(def)
			}
		}

	}

	return *settings
}

func (settings *netprobeSettings) setField(k, v string) {
	fv := reflect.ValueOf(settings).Elem().FieldByName(k)
	if fv.IsValid() {
		fv.SetString(v)
	}

}
