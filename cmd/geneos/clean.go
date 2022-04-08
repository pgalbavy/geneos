package main

import "flag"

func init() {
	RegsiterCommand(Command{
		Name:          "clean",
		Function:      commandClean,
		ParseFlags:    cleanFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos clean [-F] [TYPE] [NAME...]",
		Summary:       "Clean-up instance directory",
		Description: `Clean-up instance directories, restarting instances if doing a 'purge' clean.

FLAGS:
	-f - full clean. Stops and restarts instances. Only restarts those
	     instances the command stopped.

`,
	})

	cleanFlags = flag.NewFlagSet("clean", flag.ExitOnError)
	cleanFlags.BoolVar(&cleanPurge, "F", false, "Perform a full clean. Removes more files than basic clean and restarts instances")
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
	return Clean(c, cleanPurge, params)
}

func Clean(c Instances, purge bool, params []string) (err error) {
	var stopped bool

	cleanlist := GlobalConfig[components[c.Type()].CleanList]
	purgelist := GlobalConfig[components[c.Type()].PurgeList]

	if !purge {
		if cleanlist != "" {
			if err = deletePaths(c, cleanlist); err == nil {
				logDebug.Println(c, "cleaned")
			}
		}
		return
	}

	if _, err = findInstancePID(c); err == ErrProcNotFound {
		stopped = false
	} else if err = stopInstance(c, params); err != nil {
		return
	} else {
		stopped = true
	}

	if cleanlist != "" {
		if err = deletePaths(c, cleanlist); err != nil {
			return
		}
	}
	if purgelist != "" {
		if err = deletePaths(c, purgelist); err != nil {
			return
		}
	}
	logDebug.Println(c, "fully cleaned")
	if stopped {
		err = startInstance(c, params)
	}
	return

}
