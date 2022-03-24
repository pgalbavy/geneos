package main

import "flag"

func init() {
	RegsiterCommand(Command{
		Name:          "clean",
		Function:      commandClean,
		ParseFlags:    cleanFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos clean [-f] [TYPE] [NAME...]",
		Summary:       "Clean-up instance directory",
		Description: `Clean-up instance directories, restarting instances if doing a 'purge' clean.

FLAGS:
	-full - full clean. Stops and restarts instances. Only restarts those
	     instances the command stopped.

`,
	})

	cleanFlags = flag.NewFlagSet("clean", flag.ExitOnError)
	cleanFlags.BoolVar(&cleanPurge, "f", false, "Perform a full clean. Removes more files than basic clean and restarts instances")
	cleanFlags.BoolVar(&helpFlag, "h", false, helpUsage)
}

var cleanFlags *flag.FlagSet
var cleanPurge bool

func cleanFlag(command string, args []string) []string {
	cleanFlags.Parse(args)
	checkHelpFlag(command)
	return cleanFlags.Args()
}

func commandClean(ct Component, args []string, params []string) error {
	return ct.loopCommand(cleanInstance, args, params)
}

func cleanInstance(c Instances, params []string) (err error) {
	return c.Clean(cleanPurge, params)
}
