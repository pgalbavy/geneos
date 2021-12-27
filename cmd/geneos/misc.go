package main

import (
	"flag"
	"os"
	"sort"
)

func init() {
	commands["version"] = Command{
		Function:    commandVersion,
		ParseFlags:  defaultFlag,
		ParseArgs:   nil,
		CommandLine: "geneos version",
		Description: `Display the current version number: ` + releaseVersion}

	commands["help"] = Command{
		Function:    commandHelp,
		ParseFlags:  defaultFlag,
		ParseArgs:   nil,
		CommandLine: "geneos help [COMMAND]",
		Description: `This command. Shows either a list of available commands or the help for the given COMMAND.`}

	defaultFlags = flag.NewFlagSet("default", flag.ContinueOnError)
	defaultFlags.BoolVar(&helpFlag, "h", false, helpUsage)
}

var defaultFlags *flag.FlagSet
var helpFlag bool

const helpUsage = "Help text for command"

func commandVersion(comp ComponentType, args []string, params []string) error {
	log.Println("version:", releaseVersion)
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
		log.Printf("%s:\n\n\t%s\n\n%s", args[0], c.CommandLine, c.Description)
		return nil
	}
	return ErrInvalidArgs
}

func defaultFlag(command string, args []string) []string {
	defaultFlags.Parse(args)
	checkHelpFlag(command)
	return defaultFlags.Args()
}

// helper function to call after any parge args
// if helpFlag set, output usage of specific command and exit
func checkHelpFlag(command string) {
	if helpFlag {
		commandHelp(None, []string{command}, nil)
		os.Exit(0)
	}
}
