/*
Copyright © 2022 Peter Galbavy <peter@wonderland.org>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/component"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/internal/utils"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start [-l] [TYPE] [NAME...]",
	Short: "Start one or more instances",
	Long: `Start one or more matching instances. All instances are run in the background and
	STDOUT and STDERR are redirected to a '.txt' file in the instance directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("start called")
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().BoolVarP(&startCmdLogs, "log", "l", false, "Run 'logs -f' after starting instance(s)")
}

var startCmdLogs bool

func commandStart(ct component.ComponentType, args []string, params []string) (err error) {
	if err = instance.LoopCommand(ct, startInstance, args, params); err != nil {
		return
	}

	if startCmdLogs {
		// never returns
		return followLogs(ct, args, params)
	}
	return
}

func startInstance(c instance.Instance, params []string) (err error) {
	logDebug.Println(c, params)
	pid, err := instance.GetPID(c)
	if err == nil {
		log.Println(c, "already running with PID", pid)
		return
	}

	if IsDisabled(c) {
		return ErrDisabled
	}

	binary := c.V.GetString(c.Prefix("Exec"))
	if _, err = c.Remote().Stat(binary); err != nil {
		return fmt.Errorf("%q %w", binary, err)
	}

	cmd, env := instance.BuildCmd(c)
	if cmd == nil {
		return fmt.Errorf("buildCommand returned nil")
	}

	if !utils.CanControl(c.V.GetString(c.Prefix("User")) {
		return ErrPermission
	}

	// set underlying user for child proc
	username := c.V.GetString(c.Prefix("User"))
	errfile := instance.ConfigPathWithExt(c, "txt")

	if c.Remote() != host.LOCAL {
		r := c.Remote()
		rUsername := r.Username()
		if rUsername != username {
			return fmt.Errorf("cannot run remote process as a different user (%q != %q)", rUsername, username)
		}
		rem, err := r.Dial()
		if err != nil {
			return err
		}
		sess, err := rem.NewSession()
		if err != nil {
			return err
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
			return err
		}

		if err = sess.Shell(); err != nil {
			return err
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

		pid, err := instance.GetPID(c)
		if err != nil {
			return err
		}
		log.Println(c, "started with PID", pid)
		return nil
	}

	// pass possibly empty string down to setuser - it handles defaults
	if err = utils.SetUser(cmd, username); err != nil {
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
	// detach process by creating a session (fixed start + log)
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true

	if err = cmd.Start(); err != nil {
		return
	}
	log.Println(c, "started with PID", cmd.Process.Pid)
	if cmd.Process != nil {
		// detach from control
		cmd.Process.Release()
	}

	return
}
