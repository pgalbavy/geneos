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
	RegsiterCommand(Command{
		Name:          "start",
		Function:      commandStart,
		ParseFlags:    startFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   `geneos start [-l] [TYPE] [NAME...]`,
		Summary:       `Start one or more instances.`,
		Description: `Start one or more matching instances. All instances are run in the background and
STDOUT and STDERR are redirected to a '.txt' file in the instance directory.

FLAGS:
	-l - follow logs after starting instances`,
	})

	startFlags = flag.NewFlagSet("start", flag.ExitOnError)
	startFlags.BoolVar(&startLogs, "l", false, "Watch logs after start-up")
	startFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	RegsiterCommand(Command{
		Name:          "stop",
		Function:      commandStop,
		ParseFlags:    stopFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   `geneos stop [-f] [TYPE] [NAME...]`,
		Summary:       `Stop one or more instances`,
		Description: `Stop one or more matching instances. Unless the -f flag is given, first a SIGTERM is sent and
if the instance is still running after a few seconds then a SIGKILL is sent. If the -f flag
is given the instance(s) are immediately terminated with a SIGKILL.


FLAGS:
	-f - force stop by sending an immediate SIGKILL`,
	})

	stopFlags = flag.NewFlagSet("stop", flag.ExitOnError)
	stopFlags.BoolVar(&stopKill, "f", false, "Force stop by sending an immediate SIGKILL")
	stopFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	RegsiterCommand(Command{
		Name:          "restart",
		Function:      commandRestart,
		ParseFlags:    restartFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos restart [-l] [TYPE] [NAME...]",
		Summary:       `Restart one or more instances.`,
		Description: `Restart the matching instances. This is identical to running 'geneos stop' followed by 'geneos start'.

FLAGS:
	-l - follow logs after starting instances`,
	})

	restartFlags = flag.NewFlagSet("restart", flag.ExitOnError)
	restartFlags.BoolVar(&restartAll, "a", false, "Start all instances, not just those already running")
	restartFlags.BoolVar(&restartLogs, "l", false, "Watch logs after start-up")
	restartFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	RegsiterCommand(Command{
		Name:          "disable",
		Function:      commandDisable,
		ParseFlags:    defaultFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos disable [TYPE] [NAME...]",
		Summary:       `Disable (and stop) one or more instances.`,
		Description:   `Mark any matching instances as disabled. The instances are also stopped.`,
	})

	RegsiterCommand(Command{
		Name:          "enable",
		Function:      commandEneable,
		ParseFlags:    enableFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos enable [TYPE] [NAME...]",
		Summary:       `Enable one or more instances. Only previously disabled instances are started.`,
		Description:   `Mark any matcing instances as enabled and if this changes status then start the instance.`,
	})

	enableFlags = flag.NewFlagSet("enable", flag.ExitOnError)
	enableFlags.BoolVar(&enableNoStart, "n", false, "Do not auto-start enabled instances")
	enableFlags.BoolVar(&helpFlag, "h", false, helpUsage)
}

var stopFlags, startFlags, enableFlags *flag.FlagSet
var stopKill bool
var startLogs bool
var restartFlags *flag.FlagSet
var restartAll, restartLogs bool
var enableNoStart bool

func startFlag(command string, args []string) []string {
	startFlags.Parse(args)
	checkHelpFlag(command)
	return startFlags.Args()
}

func commandStart(ct Component, args []string, params []string) (err error) {
	if err = ct.loopCommand(startInstance, args, params); err != nil {
		return
	}
	if startLogs {
		done := make(chan bool)
		watcher, _ = watchLogs()
		defer watcher.Close()
		err = ct.loopCommand(logFollowInstance, args, params)
		<-done
	}
	return
}

func startInstance(c Instances, params []string) (err error) {
	pid, err := findInstancePID(c)
	if err == nil {
		log.Printf("%s %s@%s already running with PID %d", c.Type(), c.Name(), c.Location(), pid)

		return nil
	}

	if Disabled(c) {
		return ErrDisabled
	}

	binary := getString(c, c.Prefix("Exec"))
	if _, err = statFile(c.Location(), binary); err != nil {
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
	username := getString(c, c.Prefix("User"))
	errfile := filepath.Join(c.Home(), c.Type().String()+".txt")

	if c.Location() != LOCAL {
		r := loadRemoteConfig(c.Location())
		rUsername := getString(r, "Username")
		if rUsername != username {
			log.Fatalf("cannot run remote process as a different user (%q != %q)", rUsername, username)
		}
		rem, err := sshOpenRemote(c.Location())
		if err != nil {
			log.Fatalln(err)
		}
		sess, err := rem.NewSession()
		if err != nil {
			log.Fatalln(err)
		}

		// we have to convert cmd to a string ourselves as we have to quote any args
		// with spaces (like "Demo Gateway")
		//
		// given this is sent to a shell, we can quote everything blindly ?
		var cmdstr = ""
		for _, a := range cmd.Args {
			cmdstr = fmt.Sprintf("%s %q", cmdstr, a)
		}
		pipe, err := sess.StdinPipe()
		if err != nil {
			log.Fatalln()
		}

		if err = sess.Shell(); err != nil {
			log.Fatalln(err)
		}
		fmt.Fprintln(pipe, "cd", c.Home())
		for _, e := range env {
			fmt.Fprintln(pipe, "export", e)
		}
		fmt.Fprintf(pipe, "%s > %q 2>&1 &", cmdstr, errfile)
		fmt.Fprintln(pipe, "exit")
		sess.Close()
		// wait a short while for remote to catch-up
		time.Sleep(250 * time.Millisecond)

		pid, err := findInstancePID(c)
		if err != nil {
			log.Fatalln(err)
		}
		log.Printf("%s %s@%s started with PID %d", c.Type(), c.Name(), c.Location(), pid)
		return nil
	}

	// pass possibly empty string down to setuser - it handles defaults
	if err = setUser(cmd, username); err != nil {
		return
	}

	cmd.Env = append(os.Environ(), env...)

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
	cmd.Dir = c.Home()

	if err = cmd.Start(); err != nil {
		return
	}
	log.Printf("%s %s@%s started with PID %d", c.Type(), c.Name(), c.Location(), cmd.Process.Pid)
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

func commandStop(ct Component, args []string, params []string) (err error) {
	return ct.loopCommand(stopInstance, args, params)
}

func stopInstance(c Instances, params []string) (err error) {
	if !stopKill {
		err = signalInstance(c, syscall.SIGTERM)
		if err == ErrProcNotExist {
			return nil
		}

		if errors.Is(err, syscall.EPERM) {
			return nil
		}

		for i := 0; i < 10; i++ {
			time.Sleep(250 * time.Millisecond)
			err = signalInstance(c, syscall.SIGTERM)
			if err == ErrProcNotExist {
				break
			}
		}

		_, err = findInstancePID(c)
		if err == ErrProcNotExist {
			log.Printf("%s %s@%s stopped", c.Type(), c.Name(), c.Location())
			return nil
		}
	}

	err = signalInstance(c, syscall.SIGKILL)
	if err == ErrProcNotExist {
		return nil
	}

	time.Sleep(250 * time.Millisecond)
	_, err = findInstancePID(c)
	if err == ErrProcNotExist {
		log.Printf("%s %s@%s killed", c.Type(), c.Name(), c.Location())
		return nil
	}
	return

}

func restartFlag(command string, args []string) []string {
	restartFlags.Parse(args)
	checkHelpFlag(command)
	return restartFlags.Args()
}

func commandRestart(ct Component, args []string, params []string) (err error) {
	if err = ct.loopCommand(restartInstance, args, params); err != nil {
		logDebug.Println(err)
		return
	}

	if restartLogs {
		done := make(chan bool)
		watcher, _ = watchLogs()
		defer watcher.Close()
		err = ct.loopCommand(logFollowInstance, args, params)
		<-done
	}
	return
}

func restartInstance(c Instances, params []string) (err error) {
	err = stopInstance(c, params)
	if err == nil || (errors.Is(err, ErrProcNotExist) && restartAll) {
		return startInstance(c, params)
	}
	return
}

const disableExtension = ".disabled"

func commandDisable(ct Component, args []string, params []string) (err error) {
	return ct.loopCommand(disableInstance, args, params)
}

func disableInstance(c Instances, params []string) (err error) {
	if Disabled(c) {
		return nil
	}

	uid, gid, _, err := getUser(getString(c, c.Prefix("User")))
	if err != nil {
		return
	}

	if err = stopInstance(c, params); err != nil && !errors.Is(err, ErrProcNotExist) {
		return
	}

	disablePath := filepath.Join(c.Home(), c.Type().String()+disableExtension)

	f, err := createFile(c.Location(), disablePath, 0664)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = chown(c.Location(), disablePath, uid, gid); err != nil {
		removeFile(c.Location(), disablePath)
	}

	return
}

func enableFlag(command string, args []string) []string {
	enableFlags.Parse(args)
	checkHelpFlag(command)
	return enableFlags.Args()
}

// simpler than disable, just try to remove the flag file
// we do also start the component(s)
func commandEneable(ct Component, args []string, params []string) (err error) {
	return ct.loopCommand(enableInstance, args, params)
}

func enableInstance(c Instances, params []string) (err error) {
	err = removeFile(c.Location(), filepath.Join(c.Home(), c.Type().String()+disableExtension))
	if (err == nil || errors.Is(err, os.ErrNotExist)) && !enableNoStart {
		startInstance(c, params)
	}
	return nil
}

func Disabled(c Instances) bool {
	d := filepath.Join(c.Home(), c.Type().String()+disableExtension)
	if f, err := statFile(c.Location(), d); err == nil && f.st.Mode().IsRegular() {
		return true
	}
	return false
}
