package main

func init() {
	commands["clean"] = Command{commandClean, parseArgs, "geneos clean [TYPE] [NAME...]",
		`Clean one or more instance home directories. If a TYPE is not supplied, all instances
matching NAME(s) will be cleaned. If NAME is not supplied then all instances of the TYPE
will be cleaned. If neither TYPE or NAME is supplied all instances will be cleaned. The files
and directories removed will depend on both the TYPE.

The list of paths cannot be absolute and cannot contain parent '..' references, this is because
files and directories are removed as the user running the command, which could be root.
No further checks are done.`}

	commands["purge"] = Command{commandPurge, parseArgs, "geneos purge [TYPE] [NAME...]",
		`Stop the matching instances and remove all the files and directories that 'clean' would as well
as most other dynamically created files. The instance is not restarted.

The list of paths cannot be absolute and cannot contain parent '..' references, this is because
files and directories are removed as the user running the command, which could be root.
No further checks are done.`}
}

func commandClean(ct ComponentType, args []string) error {
	return loopCommand(cleanInstance, ct, args)
}

func cleanInstance(c Instance) (err error) {
	switch Type(c) {
	case Gateways:
		return gatewayClean(c)
	case Netprobes:
		return netprobeClean(c)
	case Licds:
		return licdClean(c)
	}
	return ErrNotSupported
}

func commandPurge(ct ComponentType, args []string) error {
	return loopCommand(purgeInstance, ct, args)
}

func purgeInstance(c Instance) (err error) {
	switch Type(c) {
	case Gateways:
		return gatewayPurge(c)
	case Netprobes:
		return netprobePurge(c)
	case Licds:
		return licdPurge(c)
	}
	return ErrNotSupported
}
