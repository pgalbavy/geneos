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
	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/instance"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start [-l] [TYPE] [NAME...]",
	Short: "Start instances",
	Long: `Start one or more matching instances. All instances are run in
the background and STDOUT and STDERR are redirected to a '.txt' file
in the instance directory. You can watch the resulting logs files with the
-l flag.`,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := cmdArgsParams(cmd)
		return commandStart(ct, startCmdLogs, args, params)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().BoolVarP(&startCmdLogs, "log", "l", false, "Run 'logs -f' after starting instance(s)")
	startCmd.Flags().SortFlags = false
}

var startCmdLogs bool

func commandStart(ct *geneos.Component, watchlogs bool, args []string, params []string) (err error) {
	if err = instance.ForAll(ct, func(c geneos.Instance, _ []string) error {
		return instance.Start(c)
	}, args, params); err != nil {
		return
	}

	if watchlogs {
		// never returns
		return followLogs(ct, args, params)
	}

	return
}
