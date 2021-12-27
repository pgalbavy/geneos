package main

import "flag"

func init() {
	commands["clean"] = Command{
		Function:    commandClean,
		ParseFlags:  cleanFlag,
		ParseArgs:   parseArgs,
		CommandLine: "geneos clean [-f] [TYPE] [NAME...]",
		Description: `Clean matching instances, stopping instances if requested for deeper cleaning.

FLAGS:
	-f - full clean. Stops and restarts instances. Only restarts instances it stopped.

`,
	}

	cleanFlags = flag.NewFlagSet("clean", flag.ExitOnError)
	cleanFlags.BoolVar(&cleanForce, "f", false, "Full clean, stops instances")
	cleanFlags.BoolVar(&helpFlag, "h", false, helpUsage)
}

var cleanFlags *flag.FlagSet
var cleanForce bool

func cleanFlag(command string, args []string) []string {
	cleanFlags.Parse(args)
	checkHelpFlag(command)
	return cleanFlags.Args()
}

func commandClean(ct ComponentType, args []string, params []string) error {
	return loopCommand(cleanInstance, ct, args, params)
}

func cleanInstance(c Instance, params []string) (err error) {
	cm, ok := components[Type(c)]
	if !ok || cm.Clean == nil {
		return ErrNotSupported
	}
	return cm.Clean(c, params)
}
