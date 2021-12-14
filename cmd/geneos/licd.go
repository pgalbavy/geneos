package main

import (
	"path/filepath"
	"strconv"
)

type Licd struct {
	Components
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

func NewLicd(name string) (c *Licd) {
	// Bootstrap
	c = &Licd{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Licds
	c.Name = name
	setDefaults(&c)
	return
}

func licdCommand(c Instance) (args, env []string) {
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

func licdClean(c Instance) (err error) {
	return removePathList(c, RunningConfig.LicdCleanList)
}

var defaultLicdPurgeList = "licd.log:licd.txt"

func licdPurge(c Instance) (err error) {
	if err = stopInstance(c); err != nil {
		return err
	}
	if err = licdClean(c); err != nil {
		return err
	}
	return removePathList(c, RunningConfig.LicdPurgeList)
}

func licdReload(c Instance) (err error) {
	return ErrNotSupported
}
