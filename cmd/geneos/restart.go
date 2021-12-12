package main

func init() {
	commands["restart"] = Command{commandRestart, parseArgs, "geneos restart [TYPE] [NAME...]",
		`Restart the matching instances. This is identical to running 'geneos stop' followed by 'geneos start'.`}
}

func commandRestart(ct ComponentType, args []string) (err error) {
	return loopCommand(restartInstance, ct, args)
}

func restartInstance(c Instance) (err error) {
	if err = stopInstance(c); err == nil {
		return startInstance(c)
	}
	return
}
