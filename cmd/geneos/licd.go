package main

import (
	"errors"
	"path/filepath"
	"strconv"
)

type Licd struct {
	Common
	BinSuffix string `default:"licd.linux_64"`
	LicdHome  string `default:"{{join .Root \"licd\" \"licds\" .Name}}"`
	LicdBins  string `default:"{{join .Root \"packages\" \"licd\"}}"`
	LicdBase  string `default:"active_prod"`
	LicdExec  string `default:"{{join .LicdBins .LicdBase .BinSuffix}}"`
	LicdLogD  string
	LicdLogF  string `default:"licd.log"`
	LicdMode  string `default:"background"`
	LicdPort  int    `default:"7041"`
	LicdOpts  string
	LicdLibs  string `default:"{{join .LicdBins .LicdBase \"lib64\"}}"`
	LicdUser  string
}

const licdPortRange = "7041,7100-"

func init() {
	components[Licds] = ComponentFuncs{
		Instance: licdInstance,
		Command:  licdCommand,
		New:      licdNew,
		Clean:    licdClean,
		Reload:   licdReload,
	}
}

func licdInstance(name string) interface{} {
	// Bootstrap
	c := &Licd{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Licds.String()
	c.Name = name
	setDefaults(&c)
	return c
}

func licdCommand(c Instance) (args, env []string) {
	args = []string{
		Name(c),
		"-port",
		getIntAsString(c, Prefix(c)+"Port"),
		"-log",
		getLogfilePath(c),
		// enable stats by default
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

func licdClean(c Instance, params []string) (err error) {
	logDebug.Println(Type(c), Name(c), "clean")
	if cleanForce {
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
