package main

import (
	"path/filepath"
	"strconv"
)

type Netprobe struct {
	Components
	NetpHome  string `default:"{{join .Root \"netprobe\" \"netprobes\" .Name}}"`
	NetpBins  string `default:"{{join .Root \"packages\" \"netprobe\"}}"`
	NetpBase  string `default:"active_prod"`
	NetpLogD  string `default:"{{.NetpHome}}"`
	NetpLogF  string `default:"netprobe.log"`
	NetpPort  int    `default:"7036"`
	NetpMode  string `default:"background"`
	NetpOpts  string // =-nopassword
	NetpLibs  string `default:"{{join .NetpBins .NetpBase \"lib64\"}}:{{join .NetpBins .NetpBase}}"`
	NetpUser  string
	BinSuffix string `default:"netprobe.linux_64"`
}

const netprobePortRange = "7036,7100-"

func NewNetprobe(name string) (c *Netprobe) {
	// Bootstrap
	c = &Netprobe{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Netprobes
	c.Name = name
	NewInstance(&c)
	return
}

func netprobeCommand(c Instance) (args, env []string) {
	logFile := filepath.Join(getString(c, Prefix(c)+"LogD"), getString(c, Prefix(c)+"LogF"))
	args = []string{
		Name(c),
		"-port",
		getIntAsString(c, Prefix(c)+"Port"),
	}
	env = append(env, "LOG_FILENAME="+logFile)
	return
}

func netprobeCreate(name string, username string) (c Instance, err error) {
	// fill in the blanks
	c = NewNetprobe(name)
	setField(c, Prefix(c)+"Port", strconv.Itoa(nextPort(RunningConfig.NetprobePortRange)))
	setField(c, Prefix(c)+"User", username)
	conffile := filepath.Join(Home(c), Type(c).String()+".json")
	writeConfigFile(conffile, c)
	// default config XML etc.
	return
}

var defaultNetprobeCleanList = "*.old"

func netprobeClean(c Instance) (err error) {
	return removePathList(c, RunningConfig.NetprobeCleanList)
}

var defaultNetprobePurgeList = "netprobe.log:netprobe.txt:*.snooze:*.user_assignment"

func netprobePurge(c Instance) (err error) {
	if err = stopInstance(c); err != nil {
		return err
	}
	if err = netprobeClean(c); err != nil {
		return err
	}
	return removePathList(c, RunningConfig.NetprobePurgeList)
}
