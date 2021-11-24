package main

type LicdComponent struct {
	Components
	LicdRoot  string `default:"{{join .ITRSHome \"licd\"}}"`
	LicdHome  string `default:"{{join .LicdRoot \"licds\" .Name}}"`
	LicdBins  string `default:"{{join .ITRSHome \"packages\" \"licd\"}}"`
	LicdBase  string `default:"active_prod"`
	LicdLogD  string `default:"{{.LicdHome}}"`
	LicdLogF  string `default:"licd.log"`
	LicdMode  string `default:"background"`
	LicdPort  int    `default:"7041"`
	LicdOpts  []string
	LicdLibs  string `default:"{{join .LicdBins .LicdBase \"lib64\"}}"`
	LicdUser  string
	BinSuffix string `default:"licd.linux_64"`
}

func NewLicd(name string) (c *LicdComponent) {
	// Bootstrap
	c = &LicdComponent{}
	c.ITRSHome = itrsHome
	c.Type = Licd
	c.Name = name
	// empty slice
	setFields(c.Components, "Opts", []string{})

	NewComponent(&c)
	return
}

func licdCmd(c Component) (args, env []string) {
	return
}
