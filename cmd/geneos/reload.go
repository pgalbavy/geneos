package main

func init() {
	commands["reload"] = Command{
		Function:    commandReload,
		ParseFlags:  defaultFlag,
		ParseArgs:   parseArgs,
		CommandLine: "geneos reload [TYPE] [NAME...]",
		Summary:     `Signal the instance to reload it's configuration, if supported.`,
		Description: `Signal the matching instances to reload their configurations, depending on the component TYPE.

Not implemented except for Gateways.`}
}

func commandReload(ct Component, args []string, params []string) error {
	return loopCommand(reloadInstance, ct, args, params)
}

func reloadInstance(c Instances, params []string) (err error) {
	// cm, ok := components[c.Type()]
	// if !ok || cm.Reload == nil {
	// 	return ErrNotSupported
	// }
	// return cm.Reload(c, params)
	return c.Reload(params)
}
