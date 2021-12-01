package main

import (
	"runtime/debug"
	"sort"
)

func init() {
	commands["version"] = commandVersion
	commands["help"] = commandHelp
}

func commandVersion(comp ComponentType, args []string) error {
	log.Println("version: unknown")
	bi, ok := debug.ReadBuildInfo()
	if ok {
		log.Printf("%+v\n", bi)
	}
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
	}
	return nil
}
