package main

type NetprobeComponent struct {
	Components
	NetpName  string
	NetpRoot  string   `default:"{{join .ITRSHome \"netprobe\"}}"`
	NetpHome  string   `default:"{{join .NetpRoot \"netprobes\" .NetpName}}"`
	NetpBins  string   `default:"{{join .ITRSHome \"packages\" \"netprobe\"}}"`
	NetpBase  string   `default:"active_prod"`
	NetpLogD  string   `default:"{{join .NetpRoot \"netprobes\"}}"`
	NetpLogF  string   `default:"netprobe.log"`
	NetpMode  string   `default:"background"`
	NetpOpts  []string // =-nopassword
	NetpLibs  string   `default:"{{join .NetpBins .NetpBase \"lib64\"}}:{{join .NetpBins .NetpBase}}"`
	NetpUser  string
	BinSuffix string `default:"netprobe.linux_64"`
}

func newNetprobe(name string) (c *NetprobeComponent) {
	// Bootstrap
	c = &NetprobeComponent{}
	c.ITRSHome = itrsHome
	c.CompType = Netprobe
	c.NetpName = name
	// empty slice
	setStringFieldSlice(c.Components, "Opts", []string{})

	newComponent(&c)
	return
}
