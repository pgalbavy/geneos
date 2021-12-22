package main

func init() {
	commands["reload"] = Command{commandReload, nil, parseArgs, "geneos reload [TYPE] [NAME...]",
		`Signal the matching instances to reload their configurations, depending on the component TYPE.

Not fully implemented except for Gateways.`}

	commands["refresh"] = Command{commandReload, nil, parseArgs, "see reload", ""}
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
