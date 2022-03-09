package main

import (
	"errors"
	"fmt"
)

const Licd Component = "licd"

type Licds struct {
	InstanceBase
	BinSuffix string `default:"licd.linux_64"`
	LicdHome  string `default:"{{join .InstanceRoot \"licd\" \"licds\" .InstanceName}}"`
	LicdBins  string `default:"{{join .InstanceRoot \"packages\" \"licd\"}}"`
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

const licdPortRange = "7041,7100-"

func init() {
	RegisterComponent(&Components{
		New:              NewLicd,
		ComponentType:    Licd,
		ComponentMatches: []string{"licd", "licds"},
		IncludeInLoops:   true,
		DownloadBase:     "Licence+Daemon",
	})
}

func NewLicd(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Licds{}
	c.InstanceRoot = remoteRoot(remote)
	c.InstanceType = Licd.String()
	c.InstanceName = local
	c.InstanceLocation = remote
	setDefaults(&c)
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

func (l Licds) Location() string {
	return l.InstanceLocation
}

func (l Licds) Home() string {
	return l.LicdHome
}

func (l Licds) Prefix(field string) string {
	return "Licd" + field
}

func (l Licds) Create(username string, params []string) (err error) {
	l.LicdPort = nextPort(l.Location(), RunningConfig.LicdPortRange)
	l.LicdUser = username

	writeInstanceConfig(l)

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(l)
	}

	// default config XML etc.
	return nil
}

func (c Licds) Command() (args, env []string) {
	args = []string{
		c.Name(),
		"-port", fmt.Sprint(c.LicdPort),
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

var defaultLicdCleanList = "*.old"
var defaultLicdPurgeList = "licd.log:licd.txt"

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
		if err = removePathList(c, RunningConfig.LicdCleanList); err != nil {
			return err
		}
		err = removePathList(c, RunningConfig.LicdPurgeList)
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return removePathList(c, RunningConfig.LicdCleanList)
}

func (c Licds) Reload(params []string) (err error) {
	return ErrNotSupported
}
