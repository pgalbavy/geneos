package main

type GatewayComponent struct {
	Components
	GateRoot  string `default:"{{join .ITRSHome \"gateway\"}}"`
	GateBins  string `default:"{{join .ITRSHome \"packages\" \"gateway\"}}"`
	GateBase  string `default:"active_prod"`
	GateLogD  string `default:"{{join .GateRoot \"gateways\"}}"`
	GateLogF  string `default:"gateway.log"`
	GateMode  string `default:"background"`
	GateLicP  string `default:"7041"`
	GateLicH  string `default:"localhost"`
	GateOpts  []string
	GateLibs  string `default:"{{join .GateBins .GateBase \"lib64\"}}:/usr/lib64"`
	GateUser  string
	BinSuffix string `default:"gateway2.linux_64"`
}

func newGateway() (c *GatewayComponent) {
	// Bootstrap
	c = &GatewayComponent{}
	c.ITRSHome = itrsHome
	c.CompType = Gateway
	// empty slice
	setStringFieldSlice(c.Components, "Opts", []string{})

	newComponent(&c)
	return
}
