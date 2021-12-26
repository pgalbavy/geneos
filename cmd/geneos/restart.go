package main

import (
	"errors"
	"flag"
)

func init() {
	commands["restart"] = Command{commandRestart, restartFlag, parseArgs, "geneos restart [TYPE] [NAME...]",
		`Restart the matching instances. This is identical to running 'geneos stop' followed by 'geneos start'.`}

	restartFlags = flag.NewFlagSet("restart", flag.ExitOnError)
	restartFlags.BoolVar(&restartAll, "a", false, "Start all instances, not just those already running")
}

var restartFlags *flag.FlagSet
var restartAll bool

func commandRestart(ct ComponentType, args []string, params []string) (err error) {
	return loopCommand(restartInstance, ct, args, params)
}

func restartFlag(args []string) []string {
	restartFlags.Parse(args)
	return restartFlags.Args()
}

func restartInstance(c Instance, params []string) (err error) {
	err = stopInstance(c, params)
	if err == nil || (errors.Is(err, ErrProcNotExist) && restartAll) {
		return startInstance(c, params)
	}
	return
}
