package main

func init() {
	commands["list"] = commandList
	commands["status"] = commandStatus
	commands["command"] = commandCommand
}

func commandList(comp ComponentType, args []string) error {
	confs := allComponents()
	for _, c := range confs {
		if comp == None || comp == Type(c) {
			log.Printf("%s => %q\n", Type(c), Name(c))
		}
	}
	return nil
}

func commandStatus(comp ComponentType, args []string) error {
	confs := allComponents()
	for _, c := range confs {
		pid, err := findProc(c)
		if err != nil {
			log.Println(Type(c), Name(c), err)
			continue
		}
		log.Println(Type(c), Name(c), "PID", pid)
	}
	return nil
}

func commandCommand(comp ComponentType, args []string) (err error) {
	for _, name := range args {
		c := New(comp, name)
		err = loadConfig(c, false)
		if err != nil {
			log.Println("cannot load configuration for", Type(c), Name(c))
			return
		}
		command(c)
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
