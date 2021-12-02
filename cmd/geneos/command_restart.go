package main

func init() {
	commands["restart"] = commandRestart
}

func commandRestart(comp ComponentType, args []string) (err error) {
	for _, name := range args {
		for _, c := range New(comp, name) {
			err = loadConfig(c, false)
			if err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				return
			}
			err = stop(c)
			if err == nil {
				start(c)
			}
		}
	}
	return
}
