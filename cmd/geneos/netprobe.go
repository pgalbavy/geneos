package main

import (
	"errors"
	"fmt"
)

const Netprobe Component = "netprobe"

type Netprobes struct {
	InstanceBase
	BinSuffix string `default:"netprobe.linux_64"`
	NetpHome  string `default:"{{join .RemoteRoot \"netprobe\" \"netprobes\" .InstanceName}}"`
	NetpBins  string `default:"{{join .RemoteRoot \"packages\" \"netprobe\"}}"`
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

func init() {
	RegisterComponent(Components{
		New:              NewNetprobe,
		ComponentType:    Netprobe,
		ComponentMatches: []string{"netprobe", "probe", "netprobes", "probes"},
		IncludeInLoops:   true,
		DownloadBase:     "Netprobe",
	})
	RegisterDirs([]string{
		"packages/netprobe",
		"netprobe/netprobes",
	})
	RegisterSettings(GlobalSettings{
		"NetprobePortRange": "7036,7100-",
		"NetprobeCleanList": "*.old",
		"NetprobePurgeList": "netprobe.log:netprobe.txt:*.snooze:*.user_assignment",
	})
}

func NewNetprobe(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Netprobes{}
	c.RemoteRoot = remoteRoot(remote)
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
	n.NetpPort = nextPort(n.Location(), GlobalConfig["NetprobePortRange"])
	n.NetpUser = username

	writeInstanceConfig(n)

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(n)
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
		if err = removePathList(c, GlobalConfig["NetprobeCleanList"]); err != nil {
			return err
		}
		err = removePathList(c, GlobalConfig["NetprobePurgeList"])
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return removePathList(c, GlobalConfig["NetprobeCleanList"])
}

func (c Netprobes) Reload(params []string) (err error) {
	return ErrNotSupported
}
