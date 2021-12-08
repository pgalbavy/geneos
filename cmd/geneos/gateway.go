package main

import "path/filepath"

type GatewayComponent struct {
	Instances
	GateHome  string `default:"{{join .Root \"gateway\" \"gateways\" .Name}}"`
	GateBins  string `default:"{{join .Root \"packages\" \"gateway\"}}"`
	GateBase  string `default:"active_prod"`
	GateLogD  string `default:"{{.GateHome}}"`
	GateLogF  string `default:"gateway.log"`
	GatePort  int    `default:"7039"`
	GateMode  string `default:"background"`
	GateLicP  int    `default:"7041"`
	GateLicH  string `default:"localhost"`
	GateOpts  string
	GateLibs  string `default:"{{join .GateBins .GateBase \"lib64\"}}:/usr/lib64"`
	GateUser  string
	BinSuffix string `default:"gateway2.linux_64"`
	// new

}

func NewGateway(name string) (c *GatewayComponent) {
	// Bootstrap
	c = &GatewayComponent{}
	c.Root = Config.ITRSHome
	c.Type = Gateway
	c.Name = name
	NewInstance(&c)
	return
}

func gatewayCmd(c Instance) (args, env []string) {
	// get opts from
	// from https://docs.itrsgroup.com/docs/geneos/5.10.0/Gateway_Reference_Guide/gateway_installation_guide.html#Gateway_command_line_options
	//
	licdhost := getString(c, Prefix(c)+"LicH")
	licdport := getIntAsString(c, Prefix(c)+"LicP")

	args = []string{
		/* "-gateway-name",  */ Name(c),
		"-resources-dir",
		filepath.Join(getString(c, Prefix(c)+"Bins"), getString(c, Prefix(c)+"Base"), "resources"),
		"-log",
		filepath.Join(getString(c, Prefix(c)+"LogD"), getString(c, Prefix(c)+"LogF")),
	}

	if licdhost != "localhost" {
		args = append(args, licdhost)
	}

	if licdport != "7041" {
		args = append(args, licdport)
	}

	return
}

/*
func createGateway(c *GatewayComponent) error {
	// fill in the blanks
	c = NewGateway(name)
}
*/
