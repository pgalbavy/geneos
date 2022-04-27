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
	"wonderland.org/geneos/internal/host"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [-b BASE] [-r REMOTE] [TYPE] VERSION",
	Short: "Update the active version of Geneos software",
	Long: `Update the symlink for the default base name of the package used to
	VERSION. The base directory, for historical reasons, is 'active_prod'
	and is usually linked to the latest version of a component type in the
	packages directory. VERSION can either be a directory name or the
	literal 'latest'. If TYPE is not supplied, all supported component
	types are updated to VERSION.

	Update will stop all matching instances of the each type before
	updating the link and starting them up again, but only if the
	instance uses the 'active_prod' basename.

	The 'latest' version is based on directory names of the form:

	[GA]X.Y.Z

	Where X, Y, Z are each ordered in ascending numerical order. If a
	directory starts 'GA' it will be selected over a directory with the
	same numerical versions. All other directories name formats will
	result in unexpected behaviour.`,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := processArgs(cmd)
		return commandUpdate(ct, args, params)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVarP(&cmdUpdateBase, "base", "b", "active_prod", "Override the base active_prod link name")
	updateCmd.Flags().StringVarP(&cmdUpdateRemote, "remote", "r", string(host.ALLHOSTS), "Perform on a remote. \"all\" - the default - means all remotes and locally")
	updateCmd.Flags().SortFlags = false
}

var cmdUpdateBase, cmdUpdateRemote string

func commandUpdate(ct *geneos.Component, args []string, params []string) (err error) {
	version := "latest"
	if len(args) > 0 {
		version = args[0]
	}
	r := host.Get(host.Name(cmdUpdateRemote))
	if err = geneos.Update(r, ct, version, cmdUpdateBase, true); err != nil && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return
}
