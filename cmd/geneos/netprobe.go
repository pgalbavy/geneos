package main

import (
	"errors"
	"strconv"
)

type Netprobes struct {
	Instance
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
	return getString(n, n.Prefix("Home"))
}

func (n Netprobes) Prefix(field string) string {
	return "Netp" + field
}

func init() {
	components[Netprobe] = ComponentFuncs{
		Instance: netprobeInstance,
		Command:  netprobeCommand,
		Add:      netprobeAdd,
		Clean:    netprobeClean,
		Reload:   netprobeReload,
	}
}

func netprobeInstance(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Netprobes{}
	c.InstanceRoot = remoteRoot(remote)
	c.InstanceType = Netprobe.String()
	c.InstanceName = local
	c.InstanceLocation = remote
	setDefaults(&c)
	return c
}

func netprobeCommand(c Instances) (args, env []string) {
	certfile := getString(c, c.Prefix("Cert"))
	keyfile := getString(c, c.Prefix("Key"))
	logFile := getLogfilePath(c)
	args = []string{
		c.Name(),
		"-port",
		getIntAsString(c, c.Prefix("Port")),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if certfile != "" {
		args = append(args, "-secure", "-ssl-certificate", certfile)
	}

	if keyfile != "" {
		args = append(args, "-ssl-certificate-key", keyfile)
	}

	return
}

// create a plain netprobe instance
func netprobeAdd(name string, username string, params []string) (c Instance, err error) {
	// fill in the blanks
	c = netprobeInstance(name).(Instance)
	netport := strconv.Itoa(nextPort(RunningConfig.NetprobePortRange))
	if netport != "7036" {
		if err = setField(c, c.Prefix("Port"), netport); err != nil {
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
	return
}

var defaultNetprobeCleanList = "*.old"
var defaultNetprobePurgeList = "netprobe.log:netprobe.txt:*.snooze:*.user_assignment"

func netprobeClean(c Instances, purge bool, params []string) (err error) {
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

func netprobeReload(c Instances, params []string) (err error) {
	return ErrNotSupported
}
