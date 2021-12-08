package main

func init() {
	commands["clean"] = Command{commandClean, parseArgs, "clean"}
}

// Clean up the working directory of a component
// Actually call a per-component function to do the work, but in general
// this removes old files, backups etc.
//
// If the component is running it should be more careful
//
func commandClean(ct ComponentType, args []string) error {
	return loopCommand(clean, ct, args)
}

func clean(c Instance) (err error) {
	return ErrNotSupported
}
