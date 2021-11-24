package main

import "path/filepath"

type GatewayComponent struct {
	Components
	GateRoot  string `default:"{{join .ITRSHome \"gateway\"}}"`
	GateHome  string `default:"{{join .GateRoot \"gateways\" .GateName}}"`
	GateBins  string `default:"{{join .ITRSHome \"packages\" \"gateway\"}}"`
	GateBase  string `default:"active_prod"`
	GateLogD  string `default:"{{join .GateRoot \"gateways\"}}"`
	GateLogF  string `default:"gateway.log"`
	GatePort  int    `default:"7039"`
	GateMode  string `default:"background"`
	GateLicP  int    `default:"7041"`
	GateLicH  string `default:"localhost"`
	GateOpts  []string
	GateLibs  string `default:"{{join .GateBins .GateBase \"lib64\"}}:/usr/lib64"`
	GateUser  string
	BinSuffix string `default:"gateway2.linux_64"`
}

func newGateway(name string) (c *GatewayComponent) {
	// Bootstrap
	c = &GatewayComponent{}
	c.ITRSHome = itrsHome
	c.Type = Gateway
	c.Name = name
	// empty slice
	setFields(c.Components, "Opts", []string{})

	newComponent(&c)
	return
}

func gatewayCmd(c Component) (args, env []string) {
	resourcesDir := filepath.Join(getStringWithPrefix(c, "Bins"), getStringWithPrefix(c, "Base"), "resources")
	logFile := filepath.Join(getStringWithPrefix(c, "LogD"), Name(c), getStringWithPrefix(c, "LogF"))
	setupFile := filepath.Join(getStringWithPrefix(c, "Home"), "gateway.setup.xml")

	args = []string{
		/* "-gateway-name",  */ Name(c),
		"-setup-file", setupFile,
		"-resources-dir", resourcesDir,
		"-log", logFile,
		"-licd-host", getStringWithPrefix(c, "LicH"),
		"-licd-port", getIntWithPrefix(c, "LicP"),
		// "-port", getIntWithPrefix(c, "Port"),
	}
	return
}
