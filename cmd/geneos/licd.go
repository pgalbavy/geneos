package main

import (
	"errors"
	"strconv"
)

type Licds struct {
	Instance
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
	return getString(l, l.Prefix("Home"))
}

func (l Licds) Prefix(field string) string {
	return "Licd" + field
}

func init() {
	components[Licd] = ComponentFuncs{
		Instance: licdInstance,
		Command:  licdCommand,
		Add:      licdAdd,
		Clean:    licdClean,
		Reload:   licdReload,
	}
}

func licdInstance(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Licds{}
	c.InstanceRoot = remoteRoot(remote)
	c.InstanceType = Licd.String()
	c.InstanceName = local
	c.InstanceLocation = remote
	setDefaults(&c)
	return c
}

func licdCommand(c Instances) (args, env []string) {
	certfile := getString(c, c.Prefix("Cert"))
	keyfile := getString(c, c.Prefix("Key"))

	args = []string{
		c.Name(),
		"-port",
		getIntAsString(c, c.Prefix("Port")),
		"-log",
		getLogfilePath(c),
	}

	if certfile != "" {
		args = append(args, "-secure", "-ssl-certificate", certfile)
	}

	if keyfile != "" {
		args = append(args, "-ssl-certificate-key", keyfile)
	}

	return
}

func licdAdd(name string, username string, params []string) (c Instance, err error) {
	// fill in the blanks
	c = licdInstance(name).(Instance)
	licdport := strconv.Itoa(nextPort(RunningConfig.LicdPortRange))
	if licdport != "7041" {
		if err = setField(c, c.Prefix("Port"), licdport); err != nil {
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

var defaultLicdCleanList = "*.old"
var defaultLicdPurgeList = "licd.log:licd.txt"

func licdClean(c Instances, purge bool, params []string) (err error) {
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

func licdReload(c Instances, params []string) (err error) {
	return ErrNotSupported
}
