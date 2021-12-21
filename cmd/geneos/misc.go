package main

import (
	"sort"
)

func init() {
	commands["version"] = Command{commandVersion, nil, nil, "geneos version",
		`Display the current version number. Currently not implmeented.`}

	commands["help"] = Command{commandHelp, nil, nil, "geneos help [COMMAND]",
		`This command. Shows either a list of available commands or the help for the given COMMAND.`}
}

func commandVersion(comp ComponentType, args []string, params []string) error {
	log.Println("version: v0.7.1")
	return nil
}

func commandHelp(comp ComponentType, args []string, params []string) error {
	if len(args) == 0 {
		keys := make([]string, 0, len(commands))
		for k := range commands {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		log.Println("The following commands are available:")
		for _, c := range keys {
			log.Printf("  %s\n   - %s", c, commands[c].CommandLine)
		}
		return nil
	}
	if c, ok := commands[args[0]]; ok {
		log.Printf("%s:\n\n\t%s\n\n%s", args[0], c.CommandLine, c.Descrtiption)
		return nil
	}
	return ErrInvalidArgs
}
