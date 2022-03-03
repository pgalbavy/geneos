package main

import (
	_ "embed"
	"errors"
	"io"
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
	GateLogD  string `json:",omitempty"`
	GateLogF  string `default:"gateway.log"`
	GatePort  int    `json:",omitempty"`
	GateMode  string `json:",omitempty"`
	GateLicP  int    `json:",omitempty"`
	GateLicH  string `json:",omitempty"`
	GateLicS  string `json:",omitempty"`
	GateOpts  string `json:",omitempty"`
	GateLibs  string `default:"{{join .GateBins .GateBase \"lib64\"}}:/usr/lib64"`
	GateUser  string `json:",omitempty"`
	GateCert  string `json:",omitempty"`
	GateKey   string `json:",omitempty"`
}

const gatewayPortRange = "7039,7100-"

//go:embed emptyGateway.xml
var emptyXMLTemplate string

func init() {
	components[Gateway] = ComponentFuncs{
		Instance: gatewayInstance,
		Command:  gatewayCommand,
		Add:      gatewayAdd,
		Clean:    gatewayClean,
		Reload:   gatewayReload,
	}
}

func gatewayInstance(name string) interface{} {
	local, remote := splitInstanceName(name)
	c := &Gateways{}
	c.Root = remoteRoot(remote)
	c.Type = Gateway.String()
	c.Name = local
	c.Location = remote
	setDefaults(&c)
	return c
}

func gatewayCommand(c Instance) (args, env []string) {
	// get opts from
	// from https://docs.itrsgroup.com/docs/geneos/5.10.0/Gateway_Reference_Guide/gateway_installation_guide.html#Gateway_command_line_options
	//
	licdhost := getString(c, Prefix(c)+"LicH")
	licdport := getIntAsString(c, Prefix(c)+"LicP")
	licdsecure := getString(c, Prefix(c)+"LicS")
	certfile := getString(c, Prefix(c)+"Cert")
	keyfile := getString(c, Prefix(c)+"Key")

	args = []string{
		/* "-gateway-name",  */ Name(c),
		"-resources-dir",
		filepath.Join(getString(c, Prefix(c)+"Bins"), getString(c, Prefix(c)+"Base"), "resources"),
		"-log",
		getLogfilePath(c),
		// enable stats by default
		"-stats",
	}

	// only add a port arg is the value is defined - empty means use config file
	port := getIntAsString(c, Prefix(c)+"Port")
	if port != "0" {
		args = append([]string{"-port", port}, args...)
	}

	if licdhost != "" {
		args = append(args, "-licd-host", licdhost)
	}

	if licdport != "0" {
		args = append(args, "-licd-port", licdport)
	}

	if licdsecure != "" && licdsecure != "false" {
		args = append(args, "-licd-secure")
	}

	if certfile != "" {
		args = append(args, "-ssl-certificate", certfile)
	}

	if keyfile != "" {
		args = append(args, "-ssl-certificate-key", keyfile)
	}

	return
}

func gatewayAdd(name string, username string, params []string) (c Instance, err error) {
	// fill in the blanks
	c = gatewayInstance(name)
	gateport := strconv.Itoa(nextPort(RunningConfig.GatewayPortRange))
	if gateport != "7039" {
		if err = setField(c, Prefix(c)+"Port", gateport); err != nil {
			return
		}
	}
	if err = setField(c, Prefix(c)+"User", username); err != nil {
		return
	}

	writeInstanceConfig(c)

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(c)
	}

	// default config XML etc.
	t, err := template.New("empty").Funcs(textJoinFuncs).Parse(emptyXMLTemplate)
	if err != nil {
		logError.Fatalln(err)
	}

	var out io.Writer

	switch RemoteName(c) {
	case LOCAL:
		cf, err := os.Create(filepath.Join(Home(c), "gateway.setup.xml"))
		out = cf
		if err != nil {
			log.Println(err)
			return nil, err
		}
		defer cf.Close()
		if err = cf.Chmod(0664); err != nil {
			logError.Fatalln(err)
		}
	default:
		cf, err := createRemoteFile(RemoteName(c), filepath.Join(Home(c), "gateway.setup.xml"))
		out = cf
		if err != nil {
			log.Println(err)
			return nil, err
		}
		defer cf.Close()
		if err = cf.Chmod(0664); err != nil {
			logError.Fatalln(err)
		}
	}

	if err = t.Execute(out, c); err != nil {
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
	if RemoteName(c) != LOCAL {
		logError.Fatalln(ErrNotSupported)
	}
	pid, _, _, _, err := findInstanceProc(c)
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
