package main

import (
	"errors"
	"fmt"
)

const Netprobe Component = "netprobe"

type Netprobes struct {
	InstanceBase
	BinSuffix string `default:"netprobe.linux_64"`
	NetpHome  string `default:"{{join .InstanceRoot \"netprobe\" \"netprobes\" .InstanceName}}"`
	NetpBins  string `default:"{{join .InstanceRoot \"packages\" \"netprobe\"}}"`
	NetpBase  string `default:"active_prod"`
	NetpExec  string `default:"{{join .NetpBins .NetpBase .BinSuffix}}"`
	NetpLogD  string `default:"{{.NetpHome}}"`
	NetpLogF  string `default:"netprobe.log"`
	NetpPort  int    `default:"7036"`
	NetpMode  string `json:",omitempty"`
	NetpOpts  string `json:",omitempty"`
	NetpLibs  string `default:"{{join .NetpBins .NetpBase \"lib64\"}}:{{join .NetpBins .NetpBase}}"`
	NetpUser  string `json:",omitempty"`
	NetpCert  string `json:",omitempty"`
	NetpKey   string `json:",omitempty"`
}

const netprobePortRange = "7036,7100-"

func init() {
	RegisterComponent(&Components{
		New:              NewNetprobe,
		ComponentType:    Netprobe,
		ComponentMatches: []string{"netprobe", "probe", "netprobes", "probes"},
		IncludeInLoops:   true,
		DownloadBase:     "Netprobe",
	})
}

func NewNetprobe(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Netprobes{}
	c.InstanceRoot = remoteRoot(remote)
	c.InstanceType = Netprobe.String()
	c.InstanceName = local
	c.InstanceLocation = remote
	setDefaults(&c)
	return c
}

// interface method set

// Return the Component for an Instance
func (n Netprobes) Type() Component {
	return parseComponentName(n.InstanceType)
}

func (n Netprobes) Name() string {
	return n.InstanceName
}

func (n Netprobes) Location() string {
	return n.InstanceLocation
}

func (n Netprobes) Home() string {
	return n.NetpHome
}

func (n Netprobes) Prefix(field string) string {
	return "Netp" + field
}

func (n Netprobes) Create(username string, params []string) (err error) {
	c := &n
	n.NetpPort = nextPort(RunningConfig.NetprobePortRange)
	n.NetpUser = username

	writeInstanceConfig(c)

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(c)
	}

	// default config XML etc.
	return nil
}

func (c Netprobes) Command() (args, env []string) {
	logFile := getLogfilePath(c)
	args = []string{
		c.Name(),
		"-port", fmt.Sprint(c.NetpPort),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if c.NetpCert != "" {
		args = append(args, "-secure", "-ssl-certificate", c.NetpCert)
	}

	if c.NetpKey != "" {
		args = append(args, "-ssl-certificate-key", c.NetpKey)
	}

	return
}

var defaultNetprobeCleanList = "*.old"
var defaultNetprobePurgeList = "netprobe.log:netprobe.txt:*.snooze:*.user_assignment"

func (c Netprobes) Clean(purge bool, params []string) (err error) {
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
		if err = removePathList(c, RunningConfig.NetprobeCleanList); err != nil {
			return err
		}
		err = removePathList(c, RunningConfig.NetprobePurgeList)
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return removePathList(c, RunningConfig.NetprobeCleanList)
}

func (c Netprobes) Reload(params []string) (err error) {
	return ErrNotSupported
}
