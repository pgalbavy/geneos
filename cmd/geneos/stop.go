package main

import (
	"errors"
	"os"
	"syscall"
	"time"
)

func init() {
	commands["stop"] = Command{commandStop, parseArgs, `geneos stop [type] [instance...]`,
		`Stop one or more instances. First a SIGTERM is sent and if the instance is still running
after a few seconds then a SIGKILL is sent to the process. If no type is given all instances with
the matching name(s) are stopped. If no instance names(s) are given then all instances of the given
type are stopped. If neither typoe or instance is given, all instances are stopped.`}

	commands["kill"] = Command{commandKill, parseArgs, `geneos kill [type] [instance...]`,
		`Immediately stop one or more instances. A SIGKILL is sent to the instance. If no type is
given all instances with the matching name(s) are killed. If no instance names(s) are given then all
instances of the given type are killed. If neither typoe or instance is given, all instances are killed.`}
}

func commandStop(ct ComponentType, args []string, params []string) (err error) {
	return loopCommand(stopInstance, ct, args, params)
}

func stopInstance(c Instance, params []string) (err error) {
	pid, st, err := findInstanceProc(c)
	if err != nil && errors.Is(err, ErrProcNotExist) {
		// not found is fine
		return nil
	}

	if !canControl(c) {
		return os.ErrPermission
	}

	DebugLog.Println("process running as", st.Uid, st.Gid)

	proc, _ := os.FindProcess(pid)

	log.Println("stopping", Type(c), Name(c), "with PID", pid)

	if err = proc.Signal(syscall.SIGTERM); err != nil {
		log.Println("sending SIGTERM failed:", err)
		return
	}

	// send a signal 0 in a loop to check if the process has terminated
	for i := 0; i < 10; i++ {
		time.Sleep(250 * time.Millisecond)
		if err = proc.Signal(syscall.Signal(0)); err != nil {
			DebugLog.Println(Type(c), "terminated")
			return nil
		}
	}

	// if still running then sigkill
	if err = proc.Signal(syscall.SIGKILL); err != nil {
		log.Println("sending SIGKILL failed:", err)
	}
	return

}

func commandKill(ct ComponentType, args []string, params []string) (err error) {
	return loopCommand(killInstance, ct, args, params)
}

func killInstance(c Instance, params []string) (err error) {
	pid, st, err := findInstanceProc(c)
	if err != nil {
		return
	}

	if !canControl(c) {
		return os.ErrPermission
	}

	DebugLog.Println("process running as", st.Uid, st.Gid)

	proc, _ := os.FindProcess(pid)

	log.Println("killing", Type(c), Name(c), "PID", pid)

	if err = proc.Signal(syscall.SIGKILL); err != nil {
		log.Println("sending SIGKILL failed:", err)
		return
	}
	return
}
