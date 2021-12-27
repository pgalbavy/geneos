package main

func init() {
	commands["reload"] = Command{
		Function:    commandReload,
		ParseFlags:  defaultFlag,
		ParseArgs:   parseArgs,
		CommandLine: "geneos reload [TYPE] [NAME...]",
		Description: `Signal the matching instances to reload their configurations, depending on the component TYPE.

Not fully implemented except for Gateways.`}
}

func commandReload(ct ComponentType, args []string, params []string) error {
	return loopCommand(reloadInstance, ct, args, params)
}

func reloadInstance(c Instance, params []string) (err error) {
	cm, ok := components[Type(c)]
	if !ok || cm.Reload == nil {
		return ErrNotSupported
	}
	return cm.Reload(c, params)
}
