package main

func init() {
	commands["clean"] = Command{commandClean, parseArgs, "geneos clean [TYPE] [NAME...]",
		`NOT YET IMPLEMENTED.
Clean one or more instance home directories. If a TYPE is not supplied, all instances
matching NAME(s) will be cleaned. If NAME is not supplied then all instances of the TYPE
will be cleaned. If neither TYPE or NAME is supplied all instances will be cleaned. The files
and directories removed will depend on both the TYPE and also if the instance is running or not.
Typically all files with an extension like .old, .bak, .history will be removed plus the cache/
and database/ directories for a stopped gateway.
`}
}

// Clean up the working directory of a component
// Actually call a per-component function to do the work, but in general
// this removes old files, backups etc.
//
// If the component is running it should be more careful
//
func commandClean(ct ComponentType, args []string) error {
	return loopCommand(cleanInstance, ct, args)
}

func cleanInstance(c Instance) (err error) {
	switch Type(c) {
	case Gateways:
		gatewayClean()
	}
	return ErrNotSupported
}
