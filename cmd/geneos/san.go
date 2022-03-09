package main

import (
	"errors"
)

const San Component = "san"

type Sans struct {
	InstanceBase
	BinSuffix string `default:"netprobe.linux_64"`
	SanHome   string `default:"{{join .InstanceRoot \"san\" \"sans\" .InstanceName}}"`
	SanBins   string `default:"{{join .InstanceRoot \"packages\" \"netprobe\"}}"`
	SanBase   string `default:"active_prod"`
	SanExec   string `default:"{{join .SanBins .SanBase .BinSuffix}}"`
	SanLogD   string `default:"{{.SanHome}}"`
	SanLogF   string `default:"san.log"`
	SanPort   int    `default:"7036"`
	SanMode   string `json:",omitempty"`
	SanOpts   string `json:",omitempty"`
	SanLibs   string `default:"{{join .SanBins .SanBase \"lib64\"}}:{{join .SanBins .SanBase}}"`
	SanUser   string `json:",omitempty"`
	SanCert   string `json:",omitempty"`
	SanKey    string `json:",omitempty"`
}

const sanPortRange = "7036,7100-"

func init() {
	RegisterComponent(&Components{
		New:              NewSan,
		ComponentType:    San,
		ComponentMatches: []string{"san", "sans"},
		IncludeInLoops:   true,
		DownloadBase:     "Netprobe",
	})
}

func NewSan(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Sans{}
	c.InstanceRoot = remoteRoot(remote)
	c.InstanceType = San.String()
	c.InstanceName = local
	c.InstanceLocation = remote
	setDefaults(&c)
	return c
}

// interface method set

// Return the Component for an Instance
func (n Sans) Type() Component {
	return parseComponentName(n.InstanceType)
}

func (n Sans) Name() string {
	return n.InstanceName
}

func (n Sans) Location() string {
	return n.InstanceLocation
}

func (n Sans) Home() string {
	return n.SanHome
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n Sans) Prefix(field string) string {
	return "San" + field
}

func (n Sans) Create(username string, params []string) (err error) {
	n.SanPort = nextPort(n.Location(), RunningConfig.SanPortRange)
	n.SanUser = username

	writeInstanceConfig(n)

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(n)
	}

	// default config XML etc.
	return nil
}

func (c Sans) Command() (args, env []string) {
	logFile := getLogfilePath(c)
	args = []string{
		c.Name(),
		"-listenip", "none",
		"-setup", "netprobe.setup.xml",
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if c.SanCert != "" {
		args = append(args, "-secure", "-ssl-certificate", c.SanCert)
	}

	if c.SanKey != "" {
		args = append(args, "-ssl-certificate-key", c.SanKey)
	}

	return
}

var defaultSanCleanList = "*.old"
var defaultSanPurgeList = "san.log:san.txt:*.snooze:*.user_assignment"

func (c Sans) Clean(purge bool, params []string) (err error) {
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
		if err = removePathList(c, RunningConfig.SanCleanList); err != nil {
			return err
		}
		err = removePathList(c, RunningConfig.SanPurgeList)
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return removePathList(c, RunningConfig.SanCleanList)
}

func (c Sans) Reload(params []string) (err error) {
	return ErrNotSupported
}
