package main

import "path/filepath"

type NetprobeComponent struct {
	Components
	NetpRoot  string `default:"{{join .Root \"netprobe\"}}"`
	NetpHome  string `default:"{{join .NetpRoot \"netprobes\" .Name}}"`
	NetpBins  string `default:"{{join .Root \"packages\" \"netprobe\"}}"`
	NetpBase  string `default:"active_prod"`
	NetpLogD  string `default:"{{.NetpHome}}"`
	NetpLogF  string `default:"netprobe.log"`
	NetpMode  string `default:"background"`
	NetpOpts  string // =-nopassword
	NetpLibs  string `default:"{{join .NetpBins .NetpBase \"lib64\"}}:{{join .NetpBins .NetpBase}}"`
	NetpUser  string
	BinSuffix string `default:"netprobe.linux_64"`
}

func NewNetprobe(name string) (c *NetprobeComponent) {
	// Bootstrap
	c = &NetprobeComponent{}
	c.Root = itrsHome
	c.Type = Netprobe
	c.Name = name
	NewComponent(&c)
	return
}

func netprobeCmd(c Component) (args, env []string) {
	logFile := filepath.Join(getString(c, Prefix(c)+"LogD"), Name(c), getString(c, Prefix(c)+"LogF"))
	env = append(env, "LOGFILE="+logFile)
	return
}
