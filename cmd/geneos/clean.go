package main

import "flag"

func init() {
	commands["clean"] = Command{
		Function:    commandClean,
		ParseFlags:  cleanFlag,
		ParseArgs:   parseArgs,
		CommandLine: "geneos clean [-f] [TYPE] [NAME...]",
		Summary:     "Clean-up instance directory",
		Description: `Clean-up instance directories, restarting instances if doing a 'purge' clean.

FLAGS:
	-p - purge / full clean. Stops and restarts instances. Only restarts those
	     instances the command stopped.

`,
	}

	cleanFlags = flag.NewFlagSet("clean", flag.ExitOnError)
	cleanFlags.BoolVar(&cleanPurge, "p", false, "Purge more files than clean, restarts instances")
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
	return loopCommand(cleanInstance, ct, args, params)
}

func cleanInstance(c Instances, params []string) (err error) {
	cm, ok := components[c.Type()]
	if !ok || cm.Clean == nil {
		return ErrNotSupported
	}
	return cm.Clean(c, cleanPurge, params)
}
