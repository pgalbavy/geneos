package main

import (
	"bufio"
	"bytes"
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

type gatewaySettings struct {
	ITRSHome  string
	GateRoot  string `default:"{{join .ITRSHome \"gateway\"}}"`
	GateBins  string `default:"{{join .ITRSHome \"packages\" \"gateway\"}}"`
	GateBase  string `default:"active_prod"`
	GateLogD  string `default:"{{join .GateRoot \"gateways\"}}"`
	GateLogF  string `default:"gateway.log"`
	GateMode  string `default:"background"`
	GateLicP  string `default:"7041"`
	GateLicH  string `default:"localhost"`
	GateOpts  []string
	GateLibs  string `default:"{{join .GateBins .GateBase \"lib64\"}}:/usr/lib64"`
	GateUser  string
	BinSuffix string `default:"gateway2.linux_64"`
}

type netprobeSettings struct {
	ITRSHome  string
	NetpRoot  string   `default:"{{join .ITRSHome \"netprobe\"}}`
	NetpBins  string   `default:"{{join .ITRSHome \"packages\" \"netprobe\"}}`
	NetpBase  string   `default:"active_prod"`
	NetpLogD  string   `default:"{{join .NetpRoot \"netprobes\"}}`
	NetpLogF  string   `default:"netprobe.log"`
	NetpMode  string   `default:"background"`
	NetpOpts  []string // =-nopassword
	NetpLibs  string   `default:"{{join .NetpBins .NetpBase \"lib64\"}}:{{join .NetpBins .NetpBase}}`
	BinSuffix string   `default:"netprobe.linux_64"`
}

var itrsHome string = "/opt/itrs"

func init() {
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		itrsHome = h
	}
}

func main() {
	gwSettings := defaultSettings()

	if len(os.Args) < 2 {
		log.Fatalln("not enough args")
	}
	switch os.Args[1] {
	case "list":
		listGateways(gwSettings)
		os.Exit(0)
	case "create":
		// createGateway()
		os.Exit(0)
	}

	if len(os.Args) < 3 {
		log.Fatalln("not enough args and lot list or create")
	}

	var gatewayName = os.Args[1]
	var action = os.Args[2]

	gateways := []string{gatewayName}
	if gatewayName == "all" {
		gateways = allGateways(gwSettings)
	}

	for _, gateway := range gateways {
		switch action {
		case "start":
			startGateway(gwSettings, gateway)
		case "stop":
			stopGateway(gwSettings, gateway)
		case "restart":
			stopGateway(gwSettings, gateway)
			startGateway(gwSettings, gateway)
		case "command":
			cmd, env := setupGateway(gwSettings, gateway)
			if cmd != nil {
				log.Printf("command: %q\n", cmd.String())
				log.Println("extra environment:")
				for _, e := range env {
					log.Println(e)
				}
			}
		case "details":
		case "status":
			pid, _, err := getPid(gwSettings, gateway)
			if err != nil {
				log.Println("gateway", gateway, "- no valid PID file found")
				continue
			}
			proc, _ := os.FindProcess(pid)
			err = proc.Signal(syscall.Signal(0))
			if err != nil {
				log.Println("gateway", gateway, "not running")
			} else {
				log.Println("gateway", gateway, "running with PID", pid)
			}
		case "refresh":
			// send a SIGUSR1
			pid, _, err := getPid(gwSettings, gateway)
			if err != nil {
				continue
			}
			proc, _ := os.FindProcess(pid)
			if err = proc.Signal(syscall.SIGUSR1); err != nil {
				log.Println("gateway", gateway, "reload config failed")
				continue
			}
		case "log":

		case "delete":

		default:
			log.Fatalln("unknown action:", action)
		}
	}
}

func listGateways(settings gatewaySettings) {
	gateways := allGateways(settings)
	for _, gateway := range gateways {
		fmt.Println(gateway)
	}
}

func allGateways(settings gatewaySettings) []string {
	gatewaysDir := filepath.Join(settings.GateRoot, "gateways")
	files, _ := os.ReadDir(gatewaysDir)
	gateways := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			gateways = append(gateways, file.Name())
		}
	}
	return gateways
}

func startGateway(settings gatewaySettings, name string) {
	cmd, env := setupGateway(settings, name)
	if cmd == nil {
		return
	}
	if len(settings.GateUser) != 0 {
		u, _ := user.Current()
		if settings.GateUser != u.Username {
			log.Println("can't change user to", settings.GateUser)
			return
		}
	}

	runGateway(settings, name, cmd, env)
}

func setupGateway(settings gatewaySettings, name string) (cmd *exec.Cmd, env []string) {
	gatewayDir := filepath.Join(settings.GateRoot, "gateways", name)
	if err := os.Chdir(gatewayDir); err != nil {
		log.Println("cannot chdir() to", gatewayDir)
		return
	}
	rcFile, err := os.Open("gateway.rc")
	if err != nil {
		log.Println("cannot open gateway.rc")
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
		case "GateOpts":
			settings.GateOpts = strings.Fields(v)
		case "BinSuffix":
			settings.setField(k, v)
		default:
			if strings.HasPrefix(k, "Gate") {
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

	gateHome := filepath.Join(settings.GateRoot, "gateways", name)
	setupFile := filepath.Join(gateHome, "gateway.setup.xml")
	binary := filepath.Join(settings.GateBins, settings.GateBase, settings.BinSuffix)
	logFile := filepath.Join(settings.GateLogD, name, settings.GateLogF)
	resources := filepath.Join(settings.GateBins, settings.GateBase, "resources")

	env = append(env, "LD_LIBRARY_PATH="+settings.GateLibs)
	args := []string{name, "-setup", setupFile, "-log", logFile, "-resources-dir", resources, "-licd-host", settings.GateLicH, "-licd-port", settings.GateLicP}
	args = append(args, settings.GateOpts...)
	cmd = exec.Command(binary, args...)

	return
}

func runGateway(settings gatewaySettings, name string, cmd *exec.Cmd, env []string) {
	gateHome := filepath.Join(settings.GateRoot, "gateways", name)

	// actually run the process
	cmd.Env = append(os.Environ(), env...)
	err := cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	if cmd.Process != nil {
		// write pid file
		pidFile := filepath.Join(gateHome, "gateway.pid")
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

func getPid(settings gatewaySettings, name string) (pid int, pidFile string, err error) {
	gateHome := filepath.Join(settings.GateRoot, "gateways", name)
	// open pid file
	pidFile = filepath.Join(gateHome, "gateway.pid")
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

func stopGateway(settings gatewaySettings, name string) {
	pid, pidFile, err := getPid(settings, name)
	if err != nil {
		//		log.Println("cannot get PID for", name)
		return
	}

	// send sigterm
	log.Println("stopping gateway", name, "with PID", pid)

	proc, _ := os.FindProcess(pid)
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		log.Println("gateway process not found, removing PID file")
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
			log.Println("gateway terminated")
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

func defaultSettings() gatewaySettings {
	settings := &gatewaySettings{}
	// Bootstrap
	settings.ITRSHome = itrsHome
	// empty slice
	settings.GateOpts = []string{}

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

func (settings *gatewaySettings) setField(k, v string) {
	fv := reflect.ValueOf(settings).Elem().FieldByName(k)
	if fv.IsValid() {
		fv.SetString(v)
	}

}
