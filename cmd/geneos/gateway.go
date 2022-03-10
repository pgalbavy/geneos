package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	// text and not html for generating XML!
)

const Gateway Component = "gateway"

type Gateways struct {
	InstanceBase
	BinSuffix string `default:"gateway2.linux_64"`
	GateHome  string `default:"{{join .RemoteRoot \"gateway\" \"gateways\" .InstanceName}}"`
	GateBins  string `default:"{{join .RemoteRoot \"packages\" \"gateway\"}}"`
	GateBase  string `default:"active_prod"`
	GateExec  string `default:"{{join .GateBins .GateBase .BinSuffix}}"`
	GateLogD  string `json:",omitempty"`
	GateLogF  string `default:"gateway.log"`
	GatePort  int    `default:"7039" json:",omitempty"`
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

//go:embed templates/gateway.setup.xml.gotmpl
var GatewayTemplate string

func init() {
	RegisterComponent(Components{
		New:              NewGateway,
		ComponentType:    Gateway,
		ComponentMatches: []string{"gateway", "gateways"},
		IncludeInLoops:   true,
		DownloadBase:     "Gateway+2",
	})
	RegisterDirs([]string{
		"packages/gateway",
		"gateway/gateways",
		"gateway/gateway_shared",
		"gateway/gateway_config",
	})
	RegisterSettings(GlobalSettings{
		"GatewayPortRange": "7039,7100-",
		"GatewayCleanList": "*.old:*.history",
		"GatewayPurgeList": "gateway.log:gateway.txt:gateway.snooze:gateway.user_assignment:licences.cache:cache/:database/",
	})
}

func NewGateway(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Gateways{}
	c.RemoteRoot = remoteRoot(remote)
	c.InstanceType = Gateway.String()
	c.InstanceName = local
	c.InstanceLocation = remote
	setDefaults(&c)
	return c
}

// interface method set

// Return the Component for an Instance
func (g Gateways) Type() Component {
	return parseComponentName(g.InstanceType)
}

func (g Gateways) Name() string {
	return g.InstanceName
}

func (g Gateways) Location() string {
	return g.InstanceLocation
}

func (g Gateways) Home() string {
	return g.GateHome
}

func (g Gateways) Prefix(field string) string {
	return "Gate" + field
}

func (g Gateways) Create(username string, params []string) (err error) {
	g.GatePort = nextPort(g.Location(), GlobalConfig["GatewayPortRange"])
	g.GateUser = username

	writeInstanceConfig(g)

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(g)
	}

	return writeTemplate(g, filepath.Join(g.Home(), "gateway.setup.xml"), GatewayTemplate)
}

func (c Gateways) Command() (args, env []string) {
	// get opts from
	// from https://docs.itrsgroup.com/docs/geneos/5.10.0/Gateway_Reference_Guide/gateway_installation_guide.html#Gateway_command_line_options
	//
	args = []string{
		/* "-gateway-name",  */ c.Name(),
		"-resources-dir",
		filepath.Join(c.GateBins, c.GateBase, "resources"),
		"-log",
		getLogfilePath(c),
		// enable stats by default
		"-stats",
	}

	// only add a port arg is the value is defined - empty means use config file
	port := c.GatePort
	if port != 7039 {
		args = append([]string{"-port", fmt.Sprint(port)}, args...)
	}

	if c.GateLicH != "" {
		args = append(args, "-licd-host", c.GateLicH)
	}

	if c.GateLicP != 0 {
		args = append(args, "-licd-port", fmt.Sprint(c.GateLicP))
	}

	if c.GateLicS != "" && c.GateLicS != "false" {
		args = append(args, "-licd-secure")
	}

	if c.GateCert != "" {
		args = append(args, "-ssl-certificate", c.GateCert)
		chainfile := filepath.Join(remoteRoot(c.Location()), "tls", "chain.pem")
		args = append(args, "-ssl-certificate-chain", chainfile)
	}

	if c.GateKey != "" {
		args = append(args, "-ssl-certificate-key", c.GateKey)
	}

	return
}

func (c Gateways) Clean(purge bool, params []string) (err error) {
	logDebug.Println(c.Type(), c.Name(), "clean")
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
		if err = removePathList(c, GlobalConfig["GatewayCleanList"]); err != nil {
			return err
		}
		err = removePathList(c, GlobalConfig["GatewayPurgeList"])
		if stopped {
			err = startInstance(c, params)
		}
		log.Printf("%s %s@%s cleaned fully", c.Type(), c.Name(), c.Location())
		return
	}
	err = removePathList(c, GlobalConfig["GatewayCleanList"])
	if err == nil {
		log.Printf("%s %s@%s cleaned", c.Type(), c.Name(), c.Location())
	}
	return
}

func (c Gateways) Reload(params []string) (err error) {
	pid, err := findInstancePID(c)
	if err != nil {
		return
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
		pipe, err := sess.StdinPipe()
		if err != nil {
			log.Fatalln()
		}

		if err = sess.Shell(); err != nil {
			log.Fatalln(err)
		}

		fmt.Fprintln(pipe, "kill -USR1", pid)
		fmt.Fprintln(pipe, "exit")
		sess.Close()

		log.Printf("%s %s@%s sent a reload signal", c.Type(), c.Name(), c.Location())
		return ErrProcExists
	}

	if !canControl(c) {
		return ErrPermission
	}

	// send a SIGUSR1
	proc, _ := os.FindProcess(pid)
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		log.Println(c.Type(), c.Name(), "refresh failed", err)

	}
	log.Printf("%s %s@%s sent a reload signal", c.Type(), c.Name(), c.Location())
	return
}
