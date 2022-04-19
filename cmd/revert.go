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

// revertCmd represents the revert command
var revertCmd = &cobra.Command{
	Use:   "revert [TYPE] [NAME...]",
	Short: "Revert migration of .rc files from backups",
	Long: `Revert migration of legacy .rc files to JSON ir the .rc.orig backup
	file still exists. Any changes to the instance configuration since
	initial migration will be lost as the contents of the .rc file is
	never changed.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("revert called")
	},
}

func init() {
	rootCmd.AddCommand(revertCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// revertCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// revertCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func commandRevert(ct *geneos.Component, names []string, params []string) (err error) {
	return instance.LoopCommand(ct, revertInstance, names, params)
}

func revertInstance(c geneos.Instance, params []string) (err error) {
	// if *.rc file exists, remove rc.orig+JSON, continue
	if _, err := c.Remote().Stat(instance.ConfigPathWithExt(c, "rc")); err == nil {
		// ignore errors
		if c.Remote().Remove(instance.ConfigPathWithExt(c, "rc.orig")) == nil || c.Remote().Remove(instance.ConfigPathWithExt(c, "json")) == nil {
			logDebug.Println(c, "removed extra config file(s)")
		}
		return err
	}

	if err = c.Remote().Rename(instance.ConfigPathWithExt(c, "rc.orig"), instance.ConfigPathWithExt(c, "rc")); err != nil {
		return
	}

	if err = c.Remote().Remove(instance.ConfigPathWithExt(c, "json")); err != nil {
		return
	}

	logDebug.Println(c, "reverted to RC config")
	return nil
}
