package main

import (
	"sort"
)

func init() {
	commands["version"] = Command{commandVersion, nil, "version"}
	commands["help"] = Command{commandHelp, nil, "help"}
}

func commandVersion(comp ComponentType, args []string) error {
	log.Println("version: undefined")
	return nil
}

func commandHelp(comp ComponentType, args []string) error {
	log.Println("help message here")
	keys := make([]string, 0, len(commands))
	for k := range commands {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	log.Println("The following commands are currently available:")
	for _, c := range keys {
		log.Println("   ", c)
		log.Println("        ", commands[c].Descrtiption)
	}
	return nil
}
