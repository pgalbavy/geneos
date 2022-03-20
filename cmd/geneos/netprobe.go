package main

import (
	"errors"
	"strconv"
)

const Netprobe Component = "netprobe"

type Netprobes struct {
	InstanceBase
	BinSuffix string `default:"netprobe.linux_64"`
	NetpHome  string `default:"{{join .RemoteRoot \"netprobe\" \"netprobes\" .InstanceName}}"`
	NetpBins  string `default:"{{join .RemoteRoot \"packages\" \"netprobe\"}}"`
	NetpBase  string `default:"active_prod"`
	NetpExec  string `default:"{{join .NetpBins .NetpBase .BinSuffix}}"`
	NetpLogD  string `json:",omitempty"`
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
		ParentType:       None,
		ComponentMatches: []string{"netprobe", "probe", "netprobes", "probes"},
		RealComponent:    true,
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
	c.InstanceRemote = GetRemote(remote)
	c.RemoteRoot = c.Remote().GeneosRoot()
	c.InstanceType = Netprobe.String()
	c.InstanceName = local
	setDefaults(&c)
	c.InstanceLocation = remote
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

func (n Netprobes) Location() RemoteName {
	return n.InstanceLocation
}

func (n Netprobes) Home() string {
	return n.NetpHome
}

func (n Netprobes) Prefix(field string) string {
	return "Netp" + field
}

func (n Netprobes) Remote() *Remotes {
	return n.InstanceRemote
}

func (n Netprobes) String() string {
	return n.Type().String() + ":" + n.InstanceName + "@" + n.Location().String()
}

func (n Netprobes) Add(username string, params []string, tmpl string) (err error) {
	n.NetpPort = n.InstanceRemote.nextPort(GlobalConfig["NetprobePortRange"])
	n.NetpUser = username

	if err = writeInstanceConfig(n); err != nil {
		logError.Fatalln(err)
	}

	// apply any extra args to settings
	if len(params) > 0 {
		commandSet(Netprobe, []string{n.Name()}, params)
		loadConfig(&n)
	}

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(&n)
	}

	// default config XML etc.
	return nil
}

func (n Netprobes) Rebuild(initial bool) error {
	return ErrNotSupported
}

func (c Netprobes) Command() (args, env []string) {
	logFile := getLogfilePath(c)
	args = []string{
		c.Name(),
		"-port", strconv.Itoa(c.NetpPort),
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
		if err = deletePaths(c, GlobalConfig["NetprobeCleanList"]); err != nil {
			return err
		}
		err = deletePaths(c, GlobalConfig["NetprobePurgeList"])
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return deletePaths(c, GlobalConfig["NetprobeCleanList"])
}

func (c Netprobes) Reload(params []string) (err error) {
	return ErrNotSupported
}
