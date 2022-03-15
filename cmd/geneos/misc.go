package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
)

func init() {
	commands["version"] = Command{
		Function:    commandVersion,
		ParseFlags:  defaultFlag,
		ParseArgs:   nil,
		CommandLine: "geneos version",
		Summary:     `Show version details.`,
		Description: `Display the current version number: ` + releaseVersion}

	commands["help"] = Command{
		Function:    commandHelp,
		ParseFlags:  defaultFlag,
		ParseArgs:   nil,
		CommandLine: "geneos help [COMMAND]",
		Summary:     `Show help text for command.`,
		Description: `This command. Shows either a list of available commands or the help for the given COMMAND.`}

	defaultFlags = flag.NewFlagSet("default", flag.ContinueOnError)
	defaultFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	commands["home"] = Command{
		Function:    commandHome,
		ParseFlags:  defaultFlag,
		ParseArgs:   nil,
		CommandLine: "geneos home [TYPE] [NAME]",
		Summary:     `Output the home directory of the installation or the first matching instance`,
		Description: `Output the home directory of the first matching instance or local
installation or the remote on stdout. This is intended for scripting,
e.g.

	cd $(geneos home)
	cd $(geneos home gateway example1
		
Because of the intended use no errors are logged and no output is
given. This would in the examples above result in the user's home
directory being selected.`}
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

func commandHome(ct Component, args []string, params []string) error {
	if len(args) == 0 {
		log.Println(ITRSHome())
		return nil
	}

	// check if first arg is a type, if not set to None else pop first arg
	if ct = parseComponentName(args[0]); ct == Unknown {
		ct = None
	} else {
		args = args[1:]
	}

	var i []Instances
	if len(args) == 0 {
		i = ct.instances(LOCAL)
	} else {
		i = ct.instanceMatches(args[0])
	}

	if len(i) == 0 {
		log.Println(ITRSHome())
		return nil
	}

	if i[0].Type() == Remote {
		log.Println(getString(i[0], "ITRSHome"))
		return nil
	}

	log.Println(i[0].Home())
	return nil
}
