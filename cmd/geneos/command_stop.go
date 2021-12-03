package main

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func init() {
	commands["stop"] = commandStop
	commands["kill"] = commandKill
}

func commandStop(comp ComponentType, args []string) (err error) {
	return loopCommand(stop, comp, args)
}

func commandKill(comp ComponentType, args []string) (err error) {
	return loopCommand(kill, comp, args)
}

func stop(c Component) (err error) {
	pid, err := findProc(c)
	if err != nil {
		return
	}

	// who is the process running as?
	s, _ := os.Stat(fmt.Sprintf("/proc/%d", pid))
	st := s.Sys().(*syscall.Stat_t)
	log.Println("process running as", st.Uid, st.Gid)

	proc, _ := os.FindProcess(pid)

	if canControl(c) {
		log.Println("stopping", Type(c), Name(c), "PID", pid)

		if err = proc.Signal(syscall.SIGTERM); err != nil {
			log.Println("sending SIGTERM failed:", err)
			return
		}

		// send a signal 0 in a loop to check if the process has terminated
		for i := 0; i < 10; i++ {
			time.Sleep(250 * time.Millisecond)
			if err = proc.Signal(syscall.Signal(0)); err != nil {
				log.Println(Type(c), "terminated")
				return nil
			}
		}
		// sigkill
		if err = proc.Signal(syscall.SIGKILL); err != nil {
			log.Println("sending SIGKILL failed:", err)
			return
		}
	} else {
		err = fmt.Errorf("cannot stop %s %s process, no permission (%v != %v)", Type(c), Name(c), st.Uid, os.Getuid())
	}
	return
}

func kill(c Component) (err error) {
	pid, err := findProc(c)
	if err != nil {
		return
	}

	// who is the process running as?
	s, _ := os.Stat(fmt.Sprintf("/proc/%d", pid))
	st := s.Sys().(*syscall.Stat_t)
	log.Println("process running as", st.Uid, st.Gid)

	proc, _ := os.FindProcess(pid)

	if canControl(c) {
		log.Println("killing", Type(c), Name(c), "PID", pid)

		// sigkill
		if err = proc.Signal(syscall.SIGKILL); err != nil {
			log.Println("sending SIGKILL failed:", err)
			return
		}
	} else {
		err = fmt.Errorf("cannot kill %s %s process, no permission (%v != %v)", Type(c), Name(c), st.Uid, os.Getuid())
	}
	return
}
