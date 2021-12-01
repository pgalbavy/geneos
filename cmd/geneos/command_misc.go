package main

func init() {
	commands["version"] = commandVersion
	commands["help"] = commandHelp
}

func commandVersion(comp ComponentType, args []string) error {
	log.Println("version X")
	return nil
}

func commandHelp(comp ComponentType, args []string) error {
	log.Println("help message here")
	return nil
}
