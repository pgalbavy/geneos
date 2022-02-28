package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// TODO: Core files and other ulimits

func init() {
	commands["start"] = Command{
		Function:    commandStart,
		ParseFlags:  startFlag,
		ParseArgs:   parseArgs,
		CommandLine: `geneos start [-l] [TYPE] [NAME...]`,
		Summary:     `Start one or more instances.`,
		Description: `Start one or more matching instances. All instances are run in the background and
STDOUT and STDERR are redirected to a '.txt' file in the instance directory.

FLAGS:
	-l - follow logs after starting instances
`}

	startFlags = flag.NewFlagSet("start", flag.ExitOnError)
	startFlags.BoolVar(&startLogs, "l", false, "Watch logs after start-up")
	startFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	commands["stop"] = Command{
		Function:    commandStop,
		ParseFlags:  stopFlag,
		ParseArgs:   parseArgs,
		CommandLine: `geneos stop [-f] [TYPE] [NAME...]`,
		Summary:     `Stop one or more instances`,
		Description: `Stop one or more matching instances. Unless the -f flag is given, first a SIGTERM is sent and
if the instance is still running after a few seconds then a SIGKILL is sent. If the -f flag
is given the instance(s) are immediately terminated with a SIGKILL.


FLAGS:
	-f - force stop by sending an immediate SIGKILL
`}

	stopFlags = flag.NewFlagSet("stop", flag.ExitOnError)
	stopFlags.BoolVar(&stopKill, "f", false, "Force stop by sending an immediate SIGKILL")
	stopFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	commands["restart"] = Command{
		Function:    commandRestart,
		ParseFlags:  restartFlag,
		ParseArgs:   parseArgs,
		CommandLine: "geneos restart [-l] [TYPE] [NAME...]",
		Summary:     `Restart one or more instances.`,
		Description: `Restart the matching instances. This is identical to running 'geneos stop' followed by 'geneos start'.

FLAGS:
	-l - follow logs after starting instances

`}

	restartFlags = flag.NewFlagSet("restart", flag.ExitOnError)
	restartFlags.BoolVar(&restartAll, "a", false, "Start all instances, not just those already running")
	restartFlags.BoolVar(&restartLogs, "l", false, "Watch logs after start-up")

	restartFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	commands["disable"] = Command{
		Function:    commandDisable,
		ParseFlags:  defaultFlag,
		ParseArgs:   parseArgs,
		CommandLine: "geneos disable [TYPE] [NAME...]",
		Summary:     `Disable (and stop) one or more instances.`,
		Description: `Mark any matching instances as disabled. The instances are also stopped.`}

	commands["enable"] = Command{
		Function:    commandEneable,
		ParseFlags:  defaultFlag,
		ParseArgs:   parseArgs,
		CommandLine: "geneos enable [TYPE] [NAME...]",
		Summary:     `Enable one or more instances. Only previously disabled instances are started.`,
		Description: `Mark any matcing instances as enabled and if this changes status then start the instance.`}
}

var stopFlags, startFlags *flag.FlagSet
var stopKill bool
var startLogs bool
var restartFlags *flag.FlagSet
var restartAll, restartLogs bool

func startFlag(command string, args []string) []string {
	startFlags.Parse(args)
	checkHelpFlag(command)
	return startFlags.Args()
}

func commandStart(ct ComponentType, args []string, params []string) (err error) {
	if err = loopCommand(startInstance, ct, args, params); err != nil {
		return
	}
	if startLogs {
		done := make(chan bool)
		watchLogs()
		defer watcher.Close()
		err = loopCommand(logFollowInstance, ct, args, params)
		<-done
	}
	return
}

func startInstance(c Instance, params []string) (err error) {
	pid, _, err := findInstanceProc(c)
	if err == nil {
		log.Println(Type(c), Name(c), "already running with PID", pid)
		return nil
	}

	if isDisabled(c) {
		return ErrDisabled
	}

	binary := getString(c, Prefix(c)+"Exec")
	if _, err = os.Stat(binary); err != nil {
		return
	}

	cmd, env := buildCmd(c)
	if cmd == nil {
		return fmt.Errorf("buildCommand returned nil")
	}

	if !canControl(c) {
		// fail early
		return ErrPermission
	}

	// set underlying user for child proc
	username := getString(c, Prefix(c)+"User")
	// pass possibly empty string down to setuser - it handles defaults
	if err = setuser(cmd, username); err != nil {
		return
	}

	cmd.Env = append(os.Environ(), env...)

	errfile := filepath.Join(Home(c), Type(c).String()+".txt")

	out, err := os.OpenFile(errfile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// if we've set-up privs at all, set the redirection output file to the same
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Credential != nil {
		if err = out.Chown(int(cmd.SysProcAttr.Credential.Uid), int(cmd.SysProcAttr.Credential.Gid)); err != nil {
			log.Println("chown:", err)
		}
	}
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Dir = Home(c)

	if err = cmd.Start(); err != nil {
		return
	}
	log.Println(Type(c), Name(c), "started with PID", cmd.Process.Pid)

	if cmd.Process != nil {
		// detach from control
		cmd.Process.Release()
	}

	return
}

func stopFlag(command string, args []string) []string {
	stopFlags.Parse(args)
	checkHelpFlag(command)
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

func restartFlag(command string, args []string) []string {
	restartFlags.Parse(args)
	checkHelpFlag(command)
	return restartFlags.Args()
}

func commandRestart(ct ComponentType, args []string, params []string) (err error) {
	if err = loopCommand(restartInstance, ct, args, params); err != nil {
		logDebug.Println(err)
		return
	}

	if restartLogs {
		done := make(chan bool)
		watchLogs()
		defer watcher.Close()
		err = loopCommand(logFollowInstance, ct, args, params)
		<-done
	}
	return
}

func restartInstance(c Instance, params []string) (err error) {
	err = stopInstance(c, params)
	if err == nil || (errors.Is(err, ErrProcNotExist) && restartAll) {
		return startInstance(c, params)
	}
	return
}

const disableExtension = ".disabled"

func commandDisable(ct ComponentType, args []string, params []string) (err error) {
	return loopCommand(disableInstance, ct, args, params)
}

func disableInstance(c Instance, params []string) (err error) {
	if isDisabled(c) {
		return ErrDisabled
	}

	uid, gid, _, err := getUser(getString(c, Prefix(c)+"User"))
	if err != nil {
		return
	}

	if err = stopInstance(c, params); err != nil && !errors.Is(err, ErrProcNotExist) {
		return
	}

	f, err := os.Create(filepath.Join(Home(c), Type(c).String()+disableExtension))
	if err != nil {
		return
	}
	defer f.Close()

	if err = f.Chown(int(uid), int(gid)); err != nil {
		os.Remove(f.Name())
	}
	return
}

// simpler than disable, just try to remove the flag file
// we do also start the component(s)
func commandEneable(ct ComponentType, args []string, params []string) (err error) {
	return loopCommand(enableInstance, ct, args, params)
}

func enableInstance(c Instance, params []string) (err error) {
	if err = os.Remove(filepath.Join(Home(c), Type(c).String()+disableExtension)); err == nil || errors.Is(err, os.ErrNotExist) {
		err = startInstance(c, params)
	}
	return
}

func isDisabled(c Instance) bool {
	d := filepath.Join(Home(c), Type(c).String()+disableExtension)
	if f, err := os.Stat(d); err == nil && f.Mode().IsRegular() {
		return true
	}
	return false
}
