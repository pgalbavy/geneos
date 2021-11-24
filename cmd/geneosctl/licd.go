package main

type LicdComponent struct {
	Components
	LicdName  string
	LicdRoot  string `default:"{{join .ITRSHome \"licd\"}}"`
	LicdHome  string `default:"{{join .LicdRoot \"licds\" .LicdName}}"`
	LicdBins  string `default:"{{join .ITRSHome \"packages\" \"licd\"}}"`
	LicdBase  string `default:"active_prod"`
	LicdLogD  string `default:"{{join .LicdRoot \"licds\"}}"`
	LicdLogF  string `default:"licd.log"`
	LicdMode  string `default:"background"`
	LicdOpts  []string
	LicdLibs  string `default:"{{join .LicdBins .LicdBase \"lib64\"}}"`
	LicdUser  string
	BinSuffix string `default:"licd.linux_64"`
}

func newLicd(name string) (c *LicdComponent) {
	// Bootstrap
	c = &LicdComponent{}
	c.ITRSHome = itrsHome
	c.Type = Licd
	c.LicdName = name
	// empty slice
	setStringFieldSlice(c.Components, "Opts", []string{})

	newComponent(&c)
	return
}
