package main

func init() {
	commands["restart"] = commandRestart
}

func commandRestart(comp ComponentType, args []string) (err error) {
	return loopCommand(restart, comp, args)
}

func restart(c Component) (err error) {
	err = stop(c)
	if err == nil {
		return start(c)
	}
	return
}
