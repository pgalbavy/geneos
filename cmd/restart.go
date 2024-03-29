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
	"errors"
	"os"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/instance"
)

// restartCmd represents the restart command
var restartCmd = &cobra.Command{
	Use:   "restart [-a] [-K] [-l] [TYPE] [NAME...]",
	Short: "Restart instances",
	Long: `Restart the matching instances. This is identical to running 'geneos
stop' followed by 'geneos start' except if the -a flag is given then
all matching instances are started regardless of whether they were
stopped by the command. The command also accepts the same flags as
both start and stop.`,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := cmdArgsParams(cmd)
		return commandRestart(ct, args, params)
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)

	restartCmd.Flags().BoolVarP(&restartCmdAll, "all", "a", false, "Start all matcheing instances, not just those already running")
	restartCmd.Flags().BoolVarP(&restartCmdKill, "kill", "K", false, "Force stop by sending an immediate SIGKILL")
	restartCmd.Flags().BoolVarP(&restartCmdLogs, "log", "l", false, "Run 'logs -f' after starting instance(s)")
	restartCmd.Flags().SortFlags = false
}

var restartCmdAll, restartCmdKill, restartCmdLogs bool

func commandRestart(ct *geneos.Component, args []string, params []string) (err error) {
	if err = instance.ForAll(ct, restartInstance, args, params); err != nil {
		logDebug.Println(err)
		return
	}

	if restartCmdLogs {
		// never returns
		return followLogs(ct, args, params)
	}
	return
}

func restartInstance(c geneos.Instance, params []string) (err error) {
	err = instance.Stop(c, false)
	if err == nil || (errors.Is(err, os.ErrProcessDone) && restartCmdAll) {
		return instance.Start(c)
	}
	return
}
