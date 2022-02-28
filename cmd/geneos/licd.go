package main

import (
	"errors"
	"path/filepath"
	"strconv"
)

type Licds struct {
	Common
	BinSuffix string `default:"licd.linux_64"`
	LicdHome  string `default:"{{join .Root \"licd\" \"licds\" .Name}}"`
	LicdBins  string `default:"{{join .Root \"packages\" \"licd\"}}"`
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
	components[Licd] = ComponentFuncs{
		Instance: licdInstance,
		Command:  licdCommand,
		New:      licdNew,
		Clean:    licdClean,
		Reload:   licdReload,
	}
}

func licdInstance(name string) interface{} {
	// Bootstrap
	c := &Licds{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Licd.String()
	c.Name = name
	setDefaults(&c)
	return c
}

func licdCommand(c Instance) (args, env []string) {
	certfile := getString(c, Prefix(c)+"Cert")
	keyfile := getString(c, Prefix(c)+"Key")

	args = []string{
		Name(c),
		"-port",
		getIntAsString(c, Prefix(c)+"Port"),
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

func licdNew(name string, username string) (c Instance, err error) {
	// fill in the blanks
	c = licdInstance(name)
	if err = setField(c, Prefix(c)+"Port", strconv.Itoa(nextPort(RunningConfig.LicdPortRange))); err != nil {
		return
	}
	if err = setField(c, Prefix(c)+"User", username); err != nil {
		return
	}
	conffile := filepath.Join(Home(c), Type(c).String()+".json")
	writeConfigFile(conffile, c)
	// default config XML etc.
	return
}

var defaultLicdCleanList = "*.old"
var defaultLicdPurgeList = "licd.log:licd.txt"

func licdClean(c Instance, purge bool, params []string) (err error) {
	logDebug.Println(Type(c), Name(c), "clean")
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

func licdReload(c Instance, params []string) (err error) {
	return ErrNotSupported
}
