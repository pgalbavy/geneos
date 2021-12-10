package main

type Licd struct {
	Components
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

const licdPortRange = "7041,7100-"

func NewLicd(name string) (c *Licd) {
	// Bootstrap
	c = &Licd{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Licds
	c.Name = name
	NewInstance(&c)
	return
}

func licdCommand(c Instance) (args, env []string) {
	return
}
