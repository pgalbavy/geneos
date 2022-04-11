package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
)

func init() {
	RegisterCommand(Command{
		Name:        "version",
		Function:    commandVersion,
		ParseFlags:  defaultFlag,
		ParseArgs:   nil,
		CommandLine: "geneos version",
		Summary:     `Show version details.`,
		Description: `Display the current version number: ` + releaseVersion,
	})

	RegisterCommand(Command{
		Name:        "help",
		Function:    commandHelp,
		ParseFlags:  defaultFlag,
		ParseArgs:   nil,
		CommandLine: "geneos help [COMMAND]",
		Summary:     `Show help text for command.`,
		Description: `This command. Shows either a list of available commands or the help for the given COMMAND.`,
	})

	defaultFlags = flag.NewFlagSet("geneos", flag.ExitOnError)
	defaultFlags.BoolVar(&helpFlag, "h", false, helpUsage)

}

var defaultFlags *flag.FlagSet
var helpFlag bool

const helpUsage = "Help text for command"

func commandVersion(comp Component, args []string, params []string) error {
	log.Println("version:", releaseVersion)
	return nil
}

func commandHelp(comp Component, args []string, params []string) error {
	if len(args) == 0 {
		keys := make([]string, 0, len(commands))
		for k := range commands {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		log.Println("The following commands are available:")
		helpTabWriter := tabwriter.NewWriter(log.Writer(), 8, 8, 4, ' ', 0)
		for _, c := range keys {
			if commands[c].Summary != "" {
				fmt.Fprintf(helpTabWriter, "\t%s\t%s\n", c, commands[c].Summary)
			} else {
				fmt.Fprintf(helpTabWriter, "\t%s\t%s\n", c, commands[c].CommandLine)
			}
		}
		helpTabWriter.Flush()
		return nil
	}
	if c, ok := commands[args[0]]; ok {
		log.Printf("\n\t%s\n\n%s", c.CommandLine, c.Description)
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
