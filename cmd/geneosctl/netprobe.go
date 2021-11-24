package main

import "path/filepath"

type NetprobeComponent struct {
	Components
	NetpRoot  string   `default:"{{join .ITRSHome \"netprobe\"}}"`
	NetpHome  string   `default:"{{join .NetpRoot \"netprobes\" .Name}}"`
	NetpBins  string   `default:"{{join .ITRSHome \"packages\" \"netprobe\"}}"`
	NetpBase  string   `default:"active_prod"`
	NetpLogD  string   `default:"{{.NetpHome}}"`
	NetpLogF  string   `default:"netprobe.log"`
	NetpMode  string   `default:"background"`
	NetpOpts  []string // =-nopassword
	NetpLibs  string   `default:"{{join .NetpBins .NetpBase \"lib64\"}}:{{join .NetpBins .NetpBase}}"`
	NetpUser  string
	BinSuffix string `default:"netprobe.linux_64"`
}

func NewNetprobe(name string) (c *NetprobeComponent) {
	// Bootstrap
	c = &NetprobeComponent{}
	c.ITRSHome = itrsHome
	c.Type = Netprobe
	c.Name = name
	// empty slice
	setFields(c.Components, "Opts", []string{})

	NewComponent(&c)
	return
}

func netprobeCmd(c Component) (args, env []string) {
	logFile := filepath.Join(getStringWithPrefix(c, "LogD"), Name(c), getStringWithPrefix(c, "LogF"))
	env = append(env, "LOGFILE="+logFile)
	return
}
