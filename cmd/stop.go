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

	"github.com/spf13/cobra"
	geneos "wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/instance"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop [-K] [TYPE] [NAME...]",
	Short: "Stop one or more instances",
	Long:  `Stop one or more matching instances. Unless the -f flag is given, first a SIGTERM is sent and if the instance is still running after a few seconds then a SIGKILL is sent. If the -f flag is given the instance(s) are immediately terminated with a SIGKILL.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("stop called")
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)

	stopCmd.Flags().BoolVarP(&stopCmdKill, "kill", "K", false, "Force immediate stop by sending an immediate SIGKILL")

}

var stopCmdKill bool

func commandStop(ct *geneos.Component, args []string, params []string) (err error) {
	return instance.LoopCommand(ct, stopInstance, args, params)
}

func stopInstance(c geneos.Instance, params []string) error {
	return instance.Stop(c, stopCmdKill, params)
}
