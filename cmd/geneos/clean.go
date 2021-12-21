package main

func init() {
	commands["clean"] = Command{commandClean, nil, parseArgs, "geneos clean [TYPE] [NAME...]",
		`Clean one or more instance home directories. If a TYPE is not supplied, all instances
matching NAME(s) will be cleaned. If NAME is not supplied then all instances of the TYPE
will be cleaned. If neither TYPE or NAME is supplied all instances will be cleaned. The files
and directories removed will depend on both the TYPE.

The list of paths cannot be absolute and cannot contain parent '..' references, this is because
files and directories are removed as the user running the command, which could be root.
No further checks are done.`}

	commands["purge"] = Command{commandPurge, nil, parseArgs, "geneos purge [TYPE] [NAME...]",
		`Stop the matching instances and remove all the files and directories that 'clean' would as well
as most other dynamically created files. The instance is not restarted.

The list of paths cannot be absolute and cannot contain parent '..' references, this is because
files and directories are removed as the user running the command, which could be root.
No further checks are done.`}
}

var cleanFuncs = perComponentFuncs{
	Gateways:  gatewayClean,
	Netprobes: netprobeClean,
	Licds:     licdClean,
}

func commandClean(ct ComponentType, args []string, params []string) error {
	return loopCommandMap(cleanFuncs, ct, args, params)
}

var purgeFuncs = perComponentFuncs{
	Gateways:  gatewayPurge,
	Netprobes: netprobePurge,
	Licds:     licdPurge,
}

func commandPurge(ct ComponentType, args []string, params []string) error {
	return loopCommandMap(purgeFuncs, ct, args, params)
}
