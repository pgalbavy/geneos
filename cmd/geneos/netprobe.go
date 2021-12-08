package main

import (
	"path/filepath"
	"strconv"
)

type NetprobeComponent struct {
	Instances
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

func NewNetprobe(name string) (c *NetprobeComponent) {
	// Bootstrap
	c = &NetprobeComponent{}
	c.Root = Config.ITRSHome
	c.Type = Netprobe
	c.Name = name
	NewInstance(&c)
	return
}

func netprobeCmd(c Instance) (args, env []string) {
	logFile := filepath.Join(getString(c, Prefix(c)+"LogD"), getString(c, Prefix(c)+"LogF"))
	args = []string{
		Name(c),
	}
	env = append(env, "LOG_FILENAME="+logFile)
	return
}

func netprobeCreate(name string, username string) (c Instance, err error) {
	// fill in the blanks
	c = NewNetprobe(name)
	setField(c, Prefix(c)+"Port", strconv.Itoa(nextPort(netprobePortRange)))
	setField(c, Prefix(c)+"User", username)
	conffile := filepath.Join(Home(c), Type(c).String()+".json")
	writeConfigFile(conffile, c)
	// default config XML etc.
	return
}
