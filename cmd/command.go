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

// commandCmd represents the command command
var commandCmd = &cobra.Command{
	Use:   "command [TYPE] [NAME...]",
	Short: "Show command arguments and environment for instances",
	Long: `Show the full command line for the matching instances along with any environment variables
	explicitly set for execution.
	
	Future releases may support CSV or JSON output formats for automation and monitoring.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("command called")
	},
}

func init() {
	rootCmd.AddCommand(commandCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// commandCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// commandCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func commandCommand(ct *geneos.Component, args []string, params []string) (err error) {
	return instance.LoopCommand(ct, commandInstance, args, params)
}

func commandInstance(c geneos.Instance, params []string) (err error) {
	log.Printf("=== %s ===", c)
	cmd, env := instance.BuildCmd(c)
	if cmd != nil {
		log.Println("command line:")
		log.Println("\t", cmd.String())
		log.Println()
		log.Println("environment:")
		for _, e := range env {
			log.Println("\t", e)
		}
		log.Println()
	}
	return
}
