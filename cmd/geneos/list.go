package main

func init() {
	commands["list"] = Command{commandList, parseArgs, "list"}
	commands["status"] = Command{commandStatus, parseArgs, "status"}
	commands["command"] = Command{commandCommand, parseArgs, "command"}
}

func commandList(ct ComponentType, args []string) error {
	return loopCommand(listInstance, ct, args)
}

func listInstance(c Instance) (err error) {
	log.Println(Type(c), Name(c))
	return
}

// also:
// user running process, maybe age (from /proc/.../status)
// show disabled/enabled status
//
// CSV and JSON versions for automation
func commandStatus(ct ComponentType, args []string) error {
	return loopCommand(statusInstance, ct, args)
}

func statusInstance(c Instance) (err error) {
	if isDisabled(c) {
		log.Println(Type(c), Name(c), ErrDisabled)
		return nil
	}
	pid, err := findProc(c)
	if err != nil {
		log.Println(Type(c), Name(c), err)
		return
	}
	log.Println(Type(c), Name(c), "PID", pid)
	return
}

func commandCommand(ct ComponentType, args []string) (err error) {
	for _, name := range args {
		for _, c := range NewComponent(ct, name) {
			if err = loadConfig(c, false); err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				return
			}
			commandInstance(c)
		}
	}
	return
}

func commandInstance(c Instance) {
	cmd, env := buildCommand(c)
	if cmd != nil {
		log.Printf("command: %q\n", cmd.String())
		log.Println("env:")
		for _, e := range env {
			log.Println(e)
		}
	}
}
