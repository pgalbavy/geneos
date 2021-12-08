package main

func init() {
	commands["restart"] = Command{commandRestart, parseArgs, "restart"}
}

func commandRestart(ct ComponentType, args []string) (err error) {
	return loopCommand(restart, ct, args)
}

func restart(c Instance) (err error) {
	err = stop(c)
	if err == nil {
		return start(c)
	}
	return
}
