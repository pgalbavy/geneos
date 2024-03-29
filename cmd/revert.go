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

// revertCmd represents the revert command
var revertCmd = &cobra.Command{
	Use:   "revert [TYPE] [NAME...]",
	Short: "Revert migration of .rc files from backups",
	Long: `Revert migration of legacy .rc files to JSON if the .rc.orig backup
file still exists. Any changes to the instance configuration since
initial migration will be lost as the contents of the .rc file is
never changed.`,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := cmdArgsParams(cmd)
		return instance.ForAll(ct, revertInstance, args, params)
	},
}

func init() {
	rootCmd.AddCommand(revertCmd)
	revertCmd.Flags().SortFlags = false
}

func revertInstance(c geneos.Instance, params []string) (err error) {
	// if *.rc file exists, remove rc.orig+JSON, continue
	if _, err := c.Host().Stat(instance.ConfigPathWithExt(c, "rc")); err == nil {
		// ignore errors
		if c.Host().Remove(instance.ConfigPathWithExt(c, "rc.orig")) == nil || c.Host().Remove(instance.ConfigPathWithExt(c, "json")) == nil {
			logDebug.Println(c, "removed extra config file(s)")
		}
		return err
	}

	if err = c.Host().Rename(instance.ConfigPathWithExt(c, "rc.orig"), instance.ConfigPathWithExt(c, "rc")); err != nil {
		return
	}

	if err = c.Host().Remove(instance.ConfigPathWithExt(c, "json")); err != nil {
		return
	}

	logDebug.Println(c, "reverted to RC config")
	return nil
}
