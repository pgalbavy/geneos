package main

func init() {
	commands["list"] = commandList
	commands["status"] = commandStatus
}

func commandList(comp ComponentType, args []string) error {
	confs := allComponents()
	for _, c := range confs {
		log.Printf("%s => %q\n", Type(c), Name(c))
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
