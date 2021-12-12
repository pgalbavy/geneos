package main

func init() {
	commands["list"] = Command{commandList, parseArgs, "geneos list [TYPE] [NAME...]",
		`List the matching instances and their component type.

Future versions will support CSV or JSON output formats for automation and monitoring.`}
	commands["status"] = Command{commandStatus, parseArgs, "geneos status [TYPE] [NAMES...]",
		`Show the status of the matching instances. This includes the component type, if it is running
and if so with what PID.

Future versions will support CSV or JSON output formats for automation and monitoring.`}
	commands["command"] = Command{commandCommand, parseArgs, "geneos command [TYPE] [NAME...]",
		`Show the full command line for the matching instances along with any environment variables
explicitly set for execution.

Future versions will support CSV or JSON output formats for automation and monitoring.`}
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
