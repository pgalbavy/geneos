package main

type LicdComponent struct {
	Instances
	LicdHome  string `default:"{{join .Root \"licd\" \"licds\" .Name}}"`
	LicdBins  string `default:"{{join .Root \"packages\" \"licd\"}}"`
	LicdBase  string `default:"active_prod"`
	LicdLogD  string `default:"{{.LicdHome}}"`
	LicdLogF  string `default:"licd.log"`
	LicdMode  string `default:"background"`
	LicdPort  int    `default:"7041"`
	LicdOpts  string
	LicdLibs  string `default:"{{join .LicdBins .LicdBase \"lib64\"}}"`
	LicdUser  string
	BinSuffix string `default:"licd.linux_64"`
}

func NewLicd(name string) (c *LicdComponent) {
	// Bootstrap
	c = &LicdComponent{}
	c.Root = Config.ITRSHome
	c.Type = Licd
	c.Name = name
	NewInstance(&c)
	return
}

func licdCmd(c Instance) (args, env []string) {
	return
}
