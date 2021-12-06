package main

func init() {
	commands["logs"] = Command{commandLogs, "logs"}
}

func commandLogs(comp ComponentType, args []string) error {
	return loopCommand(logs, comp, args)
}

func logs(c Component) (err error) {
	return ErrNotSupported
}
