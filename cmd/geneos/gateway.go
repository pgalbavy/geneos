package main

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"text/template" // text and not html for generating XML!
)

type Gateways struct {
	Common
	BinSuffix string `default:"gateway2.linux_64"`
	GateHome  string `default:"{{join .Root \"gateway\" \"gateways\" .Name}}"`
	GateBins  string `default:"{{join .Root \"packages\" \"gateway\"}}"`
	GateBase  string `default:"active_prod"`
	GateExec  string `default:"{{join .GateBins .GateBase .BinSuffix}}"`
	GateLogD  string
	GateLogF  string `default:"gateway.log"`
	GatePort  int    `default:"7039"`
	GateMode  string `default:"background"`
	GateLicP  int    `default:"7041"`
	GateLicH  string `default:"localhost"`
	GateOpts  string
	GateLibs  string `default:"{{join .GateBins .GateBase \"lib64\"}}:/usr/lib64"`
	GateUser  string
}

const gatewayPortRange = "7039,7100-"

//go:embed emptyGateway.xml
var emptyXMLTemplate string

func init() {
	components[Gateway] = ComponentFuncs{
		Instance: gatewayInstance,
		Command:  gatewayCommand,
		New:      gatewayNew,
		Clean:    gatewayClean,
		Reload:   gatewayReload,
	}
}

func gatewayInstance(name string) interface{} {
	// Bootstrap
	c := &Gateways{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Gateway.String()
	c.Name = name
	setDefaults(&c)
	return c
}

func gatewayCommand(c Instance) (args, env []string) {
	// get opts from
	// from https://docs.itrsgroup.com/docs/geneos/5.10.0/Gateway_Reference_Guide/gateway_installation_guide.html#Gateway_command_line_options
	//
	licdhost := getString(c, Prefix(c)+"LicH")
	licdport := getIntAsString(c, Prefix(c)+"LicP")

	args = []string{
		/* "-gateway-name",  */ Name(c),
		"-port",
		getIntAsString(c, Prefix(c)+"Port"),
		"-resources-dir",
		filepath.Join(getString(c, Prefix(c)+"Bins"), getString(c, Prefix(c)+"Base"), "resources"),
		"-log",
		getLogfilePath(c),
		// enable stats by default
		"-stats",
	}

	if licdhost != "localhost" {
		args = append(args, "-licd-host")
		args = append(args, licdhost)
	}

	if licdport != "7041" {
		args = append(args, "-licd-port")
		args = append(args, licdport)
	}

	return
}

func gatewayNew(name string, username string) (c Instance, err error) {
	// fill in the blanks
	c = gatewayInstance(name)
	if err = setField(c, Prefix(c)+"Port", strconv.Itoa(nextPort(RunningConfig.GatewayPortRange))); err != nil {
		return
	}
	if err = setField(c, Prefix(c)+"User", username); err != nil {
		return
	}
	conffile := filepath.Join(Home(c), Type(c).String()+".json")
	writeConfigFile(conffile, c)
	// default config XML etc.
	t, err := template.New("empty").Funcs(textJoinFuncs).Parse(emptyXMLTemplate)
	if err != nil {
		logError.Fatalln(err)
	}
	cf, err := os.OpenFile(filepath.Join(Home(c), "gateway.setup.xml"), os.O_CREATE|os.O_WRONLY, 0664)
	if err != nil {
		log.Println(err)
		return
	}
	defer cf.Close()
	if err = t.Execute(cf, c); err != nil {
		logError.Fatalln(err)
	}
	return
}

var defaultGatewayCleanList = "*.old:*.history"
var defaultGatewayPurgeList = "gateway.log:gateway.txt:gateway.snooze:gateway.user_assignment:licences.cache:cache/:database/"

func gatewayClean(c Instance, purge bool, params []string) (err error) {
	logDebug.Println(Type(c), Name(c), "clean")
	if purge {
		var stopped bool = true
		err = stopInstance(c, params)
		if err != nil {
			if errors.Is(err, ErrProcNotExist) {
				stopped = false
			} else {
				return err
			}
		}
		if err = removePathList(c, RunningConfig.GatewayCleanList); err != nil {
			return err
		}
		err = removePathList(c, RunningConfig.GatewayPurgeList)
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return removePathList(c, RunningConfig.GatewayCleanList)
}

func gatewayReload(c Instance, params []string) (err error) {
	pid, _, err := findInstanceProc(c)
	if err != nil {
		return
	}

	if !canControl(c) {
		return ErrPermission
	}

	// send a SIGUSR1
	proc, _ := os.FindProcess(pid)
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		log.Println(Type(c), Name(c), "refresh failed", err)

	}
	return
}
