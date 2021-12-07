package main

func init() {
	commands["list"] = Command{commandList, "list"}
	commands["status"] = Command{commandStatus, "status"}
	commands["command"] = Command{commandCommand, "command"}
}

func commandList(comp ComponentType, args []string) error {
	switch comp {
	case None, Unknown:
		comps := allComponents()
		for _, comp := range ComponentTypes() {
			confs, ok := comps[comp]
			if !ok {
				continue
			}
			for _, c := range confs {
				log.Printf("%s => %q\n", Type(c), Name(c))
			}
		}

	default:
		confs := components(comp)
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
func commandStatus(comp ComponentType, args []string) error {
	switch comp {
	case None, Unknown:
		comps := allComponents()
		for _, comp := range ComponentTypes() {
			confs, ok := comps[comp]
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
		confs := components(comp)
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

func commandCommand(comp ComponentType, args []string) (err error) {
	for _, name := range args {
		for _, c := range New(comp, name) {
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

func command(c Component) {
	cmd, env := buildCommand(c)
	if cmd != nil {
		log.Printf("command: %q\n", cmd.String())
		log.Println("env:")
		for _, e := range env {
			log.Println(e)
		}
	}
}
