package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"text/template" // text and not html for generating XML!

	"github.com/pkg/sftp"
)

type Gateways struct {
	InstanceBase
	BinSuffix string `default:"gateway2.linux_64"`
	GateHome  string `default:"{{join .InstanceRoot \"gateway\" \"gateways\" .InstanceName}}"`
	GateBins  string `default:"{{join .InstanceRoot \"packages\" \"gateway\"}}"`
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
	return getString(g, g.Prefix("Home"))
}

func (g Gateways) Prefix(field string) string {
	return "Gate" + field
}

func init() {
	components[Gateway] = ComponentFuncs{
		Instance: gatewayInstance,
		Command:  gatewayCommand,
		Add:      gatewayAdd,
		Clean:    gatewayClean,
		Reload:   gatewayReload,
	}
}

func gatewayInstance(name string) Instances {
	local, remote := splitInstanceName(name)
	c := new(Gateways)
	c.InstanceRoot = remoteRoot(remote)
	c.InstanceType = Gateway.String()
	c.InstanceName = local
	c.InstanceLocation = remote
	setDefaults(&c)
	return c
}

func gatewayCommand(c Instances) (args, env []string) {
	// get opts from
	// from https://docs.itrsgroup.com/docs/geneos/5.10.0/Gateway_Reference_Guide/gateway_installation_guide.html#Gateway_command_line_options
	//
	licdhost := getString(c, c.Prefix("LicH"))
	licdport := getIntAsString(c, c.Prefix("LicP"))
	licdsecure := getString(c, c.Prefix("LicS"))
	certfile := getString(c, c.Prefix("Cert"))
	keyfile := getString(c, c.Prefix("Key"))

	args = []string{
		/* "-gateway-name",  */ c.Name(),
		"-resources-dir",
		filepath.Join(getString(c, c.Prefix("Bins")), getString(c, c.Prefix("Base")), "resources"),
		"-log",
		getLogfilePath(c),
		// enable stats by default
		"-stats",
	}

	// only add a port arg is the value is defined - empty means use config file
	port := getIntAsString(c, c.Prefix("Port"))
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
		chainfile := filepath.Join(remoteRoot(c.Location()), "tls", "chain.pem")
		args = append(args, "-ssl-certificate-chain", chainfile)
	}

	if keyfile != "" {
		args = append(args, "-ssl-certificate-key", keyfile)
	}

	return
}

func gatewayAdd(name string, username string, params []string) (c Instances, err error) {
	// fill in the blanks
	c = gatewayInstance(name)
	gateport := strconv.Itoa(nextPort(RunningConfig.GatewayPortRange))
	if gateport != "7039" {
		if err = setField(c, c.Prefix("Port"), gateport); err != nil {
			return
		}
	}
	if err = setField(c, c.Prefix("User"), username); err != nil {
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

	switch c.Location() {
	case LOCAL:
		var cf *os.File
		cf, err = os.Create(filepath.Join(c.Home(), "gateway.setup.xml"))
		out = cf
		if err != nil {
			log.Println(err)
			return
		}
		defer cf.Close()
		if err = cf.Chmod(0664); err != nil {
			logError.Fatalln(err)
		}
	default:
		var cf *sftp.File
		cf, err = createRemoteFile(c.Location(), filepath.Join(c.Home(), "gateway.setup.xml"))
		out = cf
		if err != nil {
			log.Println(err)
			return
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

func gatewayClean(c Instances, purge bool, params []string) (err error) {
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
		if err = removePathList(c, RunningConfig.GatewayCleanList); err != nil {
			return err
		}
		err = removePathList(c, RunningConfig.GatewayPurgeList)
		if stopped {
			err = startInstance(c, params)
		}
		log.Printf("%s %s@%s cleaned fully", c.Type(), c.Name(), c.Location())
		return
	}
	err = removePathList(c, RunningConfig.GatewayCleanList)
	if err == nil {
		log.Printf("%s %s@%s cleaned", c.Type(), c.Name(), c.Location())
	}
	return
}

func gatewayReload(c Instances, params []string) (err error) {
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
