package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"text/template" // text and not html for generating XML!
)

type Gateway struct {
	Components
	GateHome  string `default:"{{join .Root \"gateway\" \"gateways\" .Name}}"`
	GateBins  string `default:"{{join .Root \"packages\" \"gateway\"}}"`
	GateBase  string `default:"active_prod"`
	GateLogD  string `default:"{{.GateHome}}"`
	GateLogF  string `default:"gateway.log"`
	GatePort  int    `default:"7039"`
	GateMode  string `default:"background"`
	GateLicP  int    `default:"7041"`
	GateLicH  string `default:"localhost"`
	GateOpts  string
	GateLibs  string `default:"{{join .GateBins .GateBase \"lib64\"}}:/usr/lib64"`
	GateUser  string
	BinSuffix string `default:"gateway2.linux_64"`
	// new

}

const gatewayPortRange = "7039,7100-"

//go:embed emptyGateway.xml
var emptyXMLTemplate string

func NewGateway(name string) (c *Gateway) {
	// Bootstrap
	c = &Gateway{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Gateways
	c.Name = name
	setDefaults(&c)
	return
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
		filepath.Join(getString(c, Prefix(c)+"LogD"), getString(c, Prefix(c)+"LogF")),
	}

	if licdhost != "localhost" {
		args = append(args, licdhost)
	}

	if licdport != "7041" {
		args = append(args, licdport)
	}

	return
}

func gatewayCreate(name string, username string) (c Instance, err error) {
	// fill in the blanks
	c = NewGateway(name)
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
		log.Fatalln(err)
	}
	cf, err := os.OpenFile(filepath.Join(Home(c), "gateway.setup.xml"), os.O_CREATE|os.O_WRONLY, 0664)
	if err != nil {
		log.Println(err)
		return
	}
	defer cf.Close()
	if err = t.Execute(cf, c); err != nil {
		log.Fatalln(err)
	}
	return
}

var defaultGatewayCleanList = "*.old:*.history"

func gatewayClean(c Instance) (err error) {
	return removePathList(c, RunningConfig.GatewayCleanList)
}

var defaultGatewayPurgeList = "gateway.log:gateway.txt:gateway.snooze:gateway.user_assignment:licences.cache:cache/:database/"

func gatewayPurge(c Instance) (err error) {
	if err = stopInstance(c); err != nil {
		return err
	}
	if err = gatewayClean(c); err != nil {
		return err
	}
	return removePathList(c, RunningConfig.GatewayPurgeList)
}

func gatewayReload(c Instance) (err error) {
	pid, err := findProc(c)
	if err != nil {
		return
	}

	if !canControl(c) {
		return os.ErrPermission
	}

	// send a SIGUSR1
	proc, _ := os.FindProcess(pid)
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		log.Println(Type(c), Name(c), "refresh failed")

	}
	return
}
