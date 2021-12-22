package main

import (
	"path/filepath"
	"strconv"
)

type Licd struct {
	Common
	LicdHome  string `default:"{{join .Root \"licd\" \"licds\" .Name}}"`
	LicdBins  string `default:"{{join .Root \"packages\" \"licd\"}}"`
	LicdBase  string `default:"active_prod"`
	LicdLogD  string
	LicdLogF  string `default:"licd.log"`
	LicdMode  string `default:"background"`
	LicdPort  int    `default:"7041"`
	LicdOpts  string
	LicdLibs  string `default:"{{join .LicdBins .LicdBase \"lib64\"}}"`
	LicdUser  string
	BinSuffix string `default:"licd.linux_64"`
}

const licdPortRange = "7041,7100-"

func init() {
	components[Licds] = ComponentFuncs{NewLicd}
}

func NewLicd(name string) interface{} {
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

func licdCreate(name string, username string) (c Instance, err error) {
	// fill in the blanks
	c = NewLicd(name)
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

func licdClean(c Instance, params []string) (err error) {
	return removePathList(c, RunningConfig.LicdCleanList)
}

var defaultLicdPurgeList = "licd.log:licd.txt"

func licdPurge(c Instance, params []string) (err error) {
	if err = stopInstance(c, params); err != nil {
		return err
	}
	if err = licdClean(c, params); err != nil {
		return err
	}
	return removePathList(c, RunningConfig.LicdPurgeList)
}

func licdReload(c Instance, params []string) (err error) {
	return ErrNotSupported
}
