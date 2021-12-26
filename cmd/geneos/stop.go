package main

import (
	"errors"
	"flag"
	"os"
	"syscall"
	"time"
)

func init() {
	commands["stop"] = Command{commandStop, stopFlag, parseArgs, `geneos stop [type] [instance...]`,
		`Stop one or more matching instances. First a SIGTERM is sent and if the instance is still running
after a few seconds then a SIGKILL is sent to the process(es).`}

	stopFlags = flag.NewFlagSet("stop", flag.ExitOnError)
	stopFlags.BoolVar(&stopKill, "f", false, "Force stop by senind an immediate SIGKILL")
}

var stopFlags *flag.FlagSet
var stopKill bool

func stopFlag(args []string) []string {
	stopFlags.Parse(args)
	return stopFlags.Args()
}

func commandStop(ct ComponentType, args []string, params []string) (err error) {
	return loopCommand(stopInstance, ct, args, params)
}

func stopInstance(c Instance, params []string) (err error) {
	pid, st, err := findInstanceProc(c)
	if err != nil && errors.Is(err, ErrProcNotExist) {
		// not found is fine
		return
	}

	if !canControl(c) {
		return ErrPermission
	}

	logDebug.Println("process running as", st.Uid, st.Gid)

	proc, _ := os.FindProcess(pid)

	if !stopKill {
		log.Println("stopping", Type(c), Name(c), "with PID", pid)

		if err = proc.Signal(syscall.SIGTERM); err != nil {
			log.Println("sending SIGTERM failed:", err)
			return
		}

		// send a signal 0 in a loop to check if the process has terminated
		for i := 0; i < 10; i++ {
			time.Sleep(250 * time.Millisecond)
			if err = proc.Signal(syscall.Signal(0)); err != nil {
				logDebug.Println(Type(c), "terminated")
				return nil
			}
		}
	}

	// if -f or still running then sigkill
	if err = proc.Signal(syscall.SIGKILL); err != nil {
		log.Println("sending SIGKILL failed:", err)
	}
	log.Println("killed", Type(c), Name(c), "with PID", pid)
	return

}
