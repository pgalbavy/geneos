package main

import "path/filepath"

type GatewayComponent struct {
	Components
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
}

func NewGateway(name string) (c *GatewayComponent) {
	// Bootstrap
	c = &GatewayComponent{}
	c.Root = itrsHome
	c.Type = Gateway
	c.Name = name
	NewComponent(&c)
	return
}

func gatewayCmd(c Component) (args, env []string) {
	resourcesDir := filepath.Join(getString(c, Prefix(c)+"Bins"), getString(c, Prefix(c)+"Base"), "resources")
	logFile := filepath.Join(getString(c, Prefix(c)+"LogD"), Name(c), getString(c, Prefix(c)+"LogF"))
	setupFile := filepath.Join(getString(c, Prefix(c)+"Home"), "gateway.setup.xml")

	args = []string{
		/* "-gateway-name",  */ Name(c),
		"-setup-file", setupFile,
		"-resources-dir", resourcesDir,
		"-log", logFile,
		"-licd-host", getString(c, Prefix(c)+"LicH"),
		"-licd-port", getInt(c, Prefix(c)+"LicP"),
		// "-port", getIntWithPrefix(c, "Port"),
	}

	return
}

/*
func createGateway(c *GatewayComponent) error {
	// fill in the blanks
	c = NewGateway(name)
}
*/
