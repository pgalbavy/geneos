package main

import (
	"errors"
	"strconv"
)

const Licd Component = "licd"

type Licds struct {
	InstanceBase
	BinSuffix string `default:"licd.linux_64"`
	LicdHome  string `default:"{{join .RemoteRoot \"licd\" \"licds\" .InstanceName}}"`
	LicdBins  string `default:"{{join .RemoteRoot \"packages\" \"licd\"}}"`
	LicdBase  string `default:"active_prod"`
	LicdExec  string `default:"{{join .LicdBins .LicdBase .BinSuffix}}"`
	LicdLogD  string `json:",omitempty"`
	LicdLogF  string `default:"licd.log"`
	LicdMode  string `json:",omitempty"`
	LicdPort  int    `default:"7041"`
	LicdOpts  string `json:",omitempty"`
	LicdLibs  string `default:"{{join .LicdBins .LicdBase \"lib64\"}}"`
	LicdUser  string `json:",omitempty"`
	LicdCert  string `json:",omitempty"`
	LicdKey   string `json:",omitempty"`
}

func init() {
	RegisterComponent(Components{
		New:              NewLicd,
		ComponentType:    Licd,
		ParentType:       None,
		ComponentMatches: []string{"licd", "licds"},
		RealComponent:    true,
		DownloadBase:     "Licence+Daemon",
	})
	RegisterDirs([]string{
		"packages/licd",
		"licd/licds",
	})
	RegisterSettings(GlobalSettings{
		"LicdPortRange": "7041,7100-",
		"LicdCleanList": "*.old",
		"LicdPurgeList": "licd.log:licd.txt",
	})
}

func NewLicd(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Licds{}
	c.RemoteRoot = GeneosRoot(remote)
	c.InstanceType = Licd.String()
	c.InstanceName = local
	setDefaults(&c)
	c.InstanceLocation = remote
	c.InstanceRemote = loadRemoteConfig(remote)
	return c
}

// interface method set

// Return the Component for an Instance
func (l Licds) Type() Component {
	return parseComponentName(l.InstanceType)
}

func (l Licds) Name() string {
	return l.InstanceName
}

func (l Licds) Location() RemoteName {
	return l.InstanceLocation
}

func (l Licds) Home() string {
	return l.LicdHome
}

func (l Licds) Prefix(field string) string {
	return "Licd" + field
}

func (l Licds) Add(username string, params []string, tmpl string) (err error) {
	l.LicdPort = l.InstanceRemote.nextPort(GlobalConfig["LicdPortRange"])
	l.LicdUser = username

	if err = writeInstanceConfig(l); err != nil {
		logError.Fatalln(err)
	}

	// apply any extra args to settings
	if len(params) > 0 {
		commandSet(Licd, []string{l.Name()}, params)
		loadConfig(&l)
	}

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(&l)
	}

	// default config XML etc.
	return nil
}

func (l Licds) Rebuild(initial bool) error {
	return ErrNotSupported
}

func (c Licds) Command() (args, env []string) {
	args = []string{
		c.Name(),
		"-port", strconv.Itoa(c.LicdPort),
		"-log", getLogfilePath(c),
	}

	if c.LicdCert != "" {
		args = append(args, "-secure", "-ssl-certificate", c.LicdCert)
	}

	if c.LicdKey != "" {
		args = append(args, "-ssl-certificate-key", c.LicdKey)
	}

	return
}

func (c Licds) Clean(purge bool, params []string) (err error) {
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
		if err = deletePaths(c, GlobalConfig["LicdCleanList"]); err != nil {
			return err
		}
		err = deletePaths(c, GlobalConfig["LicdPurgeList"])
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return deletePaths(c, GlobalConfig["LicdCleanList"])
}

func (c Licds) Reload(params []string) (err error) {
	return ErrNotSupported
}
