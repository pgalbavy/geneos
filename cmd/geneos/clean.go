package main

func init() {
	commands["clean"] = Command{
		Function:    commandClean,
		ParseFlags:  nil,
		ParseArgs:   parseArgs,
		CommandLine: "geneos clean [TYPE] [NAME...]",
		Description: `Clean one or more instance home directories. If a TYPE is not supplied, all instances
matching NAME(s) will be cleaned. If NAME is not supplied then all instances of the TYPE
will be cleaned. If neither TYPE or NAME is supplied all instances will be cleaned. The files
and directories removed will depend on both the TYPE.

The list of paths cannot be absolute and cannot contain parent '..' references, this is because
files and directories are removed as the user running the command, which could be root.
No further checks are done.`,
	}

	commands["purge"] = Command{commandPurge, nil, parseArgs, "geneos purge [TYPE] [NAME...]",
		`Stop the matching instances and remove all the files and directories that 'clean' would as well
as most other dynamically created files. The instance is not restarted.

The list of paths cannot be absolute and cannot contain parent '..' references, this is because
files and directories are removed as the user running the command, which could be root.
No further checks are done.`}
}

func commandClean(ct ComponentType, args []string, params []string) error {
	return loopCommand(cleanInstance, ct, args, params)
}

func cleanInstance(c Instance, params []string) (err error) {
	cm, ok := components[Type(c)]
	if !ok || cm.Clean == nil {
		return ErrNotSupported
	}
	return cm.Clean(c, params)
}

func commandPurge(ct ComponentType, args []string, params []string) error {
	return loopCommand(purgeInstance, ct, args, params)
}

func purgeInstance(c Instance, params []string) (err error) {
	cm, ok := components[Type(c)]
	if !ok || cm.Purge == nil {
		return ErrNotSupported
	}
	return cm.Purge(c, params)
}
