package main

func init() {
	commands["reload"] = Command{commandReload, parseArgs, "geneos reload [TYPE] [NAME...]",
		`Signal the matching instances to reload their configurations, depending on the component TYPE.

Not fully implemented except for Gateways.`}

	commands["refresh"] = Command{commandReload, parseArgs, "see reload", ""}
}

var reloadFuncs = perComponentFuncs{
	Gateways:  gatewayReload,
	Netprobes: netprobeReload,
	Licds:     licdReload,
}

func commandReload(ct ComponentType, args []string, params []string) error {
	return loopCommandMap(reloadFuncs, ct, args, params)
}
