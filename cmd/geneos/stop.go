package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

func init() {
	commands["stop"] = Command{commandStop, parseArgs, `geneos stop [type] [instance...]`,
		`Stop one or more instances. First a SIGTERM is sent and if the instance is still running
	after a few seconds then a SIGKILL is sent to the process.
	If no type is given all instances with the matching name(s) are stopped.
	If no instance names(s) are given then all instances of the given type are stopped.
	If neither typoe or instance is given, all instances are stopped.
`}
	commands["kill"] = Command{commandKill, parseArgs, `geneos kill [type] [instance...]`,
		`Immediately stop one or more instances. A SIGKILL is sent to the instance.
	If no type is given all instances with the matching name(s) are killed.
	If no instance names(s) are given then all instances of the given type are killed.
	If neither typoe or instance is given, all instances are killed.
`}
}

func commandStop(ct ComponentType, args []string) (err error) {
	return loopCommand(stopInstance, ct, args)
}

func commandKill(ct ComponentType, args []string) (err error) {
	return loopCommand(kill, ct, args)
}

func stopInstance(c Instance) (err error) {
	pid, err := findProc(c)
	if err != nil && errors.Is(err, ErrProcNotExist) {
		// not found is fine
		return nil
	}

	// who is the process running as?
	s, _ := os.Stat(fmt.Sprintf("/proc/%d", pid))
	st := s.Sys().(*syscall.Stat_t)
	DebugLog.Println("process running as", st.Uid, st.Gid)

	proc, _ := os.FindProcess(pid)

	if !canControl(c) {
		return os.ErrPermission
	}

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

func kill(c Instance) (err error) {
	pid, err := findProc(c)
	if err != nil {
		return
	}

	// who is the process running as?
	s, _ := os.Stat(fmt.Sprintf("/proc/%d", pid))
	st := s.Sys().(*syscall.Stat_t)
	log.Println("process running as", st.Uid, st.Gid)

	proc, _ := os.FindProcess(pid)

	if !canControl(c) {
		return os.ErrPermission
	}

	log.Println("killing", Type(c), Name(c), "PID", pid)

	// sigkill
	if err = proc.Signal(syscall.SIGKILL); err != nil {
		log.Println("sending SIGKILL failed:", err)
		return
	}
	return
}
