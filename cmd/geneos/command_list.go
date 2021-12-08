package main

func init() {
	commands["list"] = Command{commandList, parseArgs, "list"}
	commands["status"] = Command{commandStatus, parseArgs, "status"}
	commands["command"] = Command{commandCommand, parseArgs, "command"}
}

func commandList(ct ComponentType, args []string) error {
	switch ct {
	case None, Unknown:
		comps := allInstances()
		for _, cts := range ComponentTypes() {
			confs, ok := comps[cts]
			if !ok {
				continue
			}
			for _, c := range confs {
				log.Printf("%s => %q\n", Type(c), Name(c))
			}
		}

	default:
		confs := instances(ct)
		for _, c := range confs {
			log.Printf("%s => %q\n", Type(c), Name(c))
		}
	}
	return nil
}

// also:
// user running process, maybe age (from /proc/.../status)
// show disabled/enabled status
//
// CSV and JSON versions for automation
func commandStatus(ct ComponentType, args []string) error {
	switch ct {
	case None, Unknown:
		comps := allInstances()
		for _, cts := range ComponentTypes() {
			confs, ok := comps[cts]
			if !ok {
				continue
			}
			for _, c := range confs {
				if isDisabled(c) {
					log.Println(Type(c), Name(c), ErrDisabled)
					continue
				}
				pid, err := findProc(c)
				if err != nil {
					log.Println(Type(c), Name(c), err)
					continue
				}
				log.Println(Type(c), Name(c), "PID", pid)
			}
		}

	default:
		confs := instances(ct)
		for _, c := range confs {
			if isDisabled(c) {
				log.Println(Type(c), Name(c), ErrDisabled)
				continue
			}
			pid, err := findProc(c)
			if err != nil {
				log.Println(Type(c), Name(c), err)
				continue
			}
			log.Println(Type(c), Name(c), "PID", pid)
		}
	}
	return nil
}

func commandCommand(ct ComponentType, args []string) (err error) {
	for _, name := range args {
		for _, c := range New(ct, name) {
			err = loadConfig(c, false)
			if err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				return
			}
			command(c)
		}
	}
	return
}

func command(c Instance) {
	cmd, env := buildCommand(c)
	if cmd != nil {
		log.Printf("command: %q\n", cmd.String())
		log.Println("env:")
		for _, e := range env {
			log.Println(e)
		}
	}
}
