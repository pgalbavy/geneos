package main

import "errors"

func init() {
	commands["restart"] = Command{commandRestart, nil, parseArgs, "geneos restart [TYPE] [NAME...]",
		`Restart the matching instances. This is identical to running 'geneos stop' followed by 'geneos start'.`}
}

func commandRestart(ct ComponentType, args []string, params []string) (err error) {
	return loopCommand(restartInstance, ct, args, params)
}

func restartInstance(c Instance, params []string) (err error) {
	if err = stopInstance(c, params); err == nil || errors.Is(err, ErrProcNotExist) {
		return startInstance(c, params)
	}
	return
}
