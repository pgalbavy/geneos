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

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [-F] [TYPE] [NAME...]",
	Short: "Delete an instance. Instance must be stopped",
	Long: `Delete the matching instances. This will only work on
instances that are disabled to prevent accidental deletion. The
instance directory is removed without being backed-up. The user
running the command must have the appropriate permissions and a
partial deletion cannot be protected against.`,
	SilenceUsage: true,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := processArgs(cmd)
		return instance.ForAll(ct, deleteInstance, args, params)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.Flags().BoolVarP(&deleteCmdForce, "force", "F", false, "Force delete of instances")
}

var deleteCmdForce bool

func deleteInstance(c geneos.Instance, params []string) (err error) {
	if deleteCmdForce {
		if c.Type().RealComponent {
			if err = instance.Stop(c, false, nil); err != nil {
				return
			}
		}
	}

	if deleteCmdForce || instance.IsDisabled(c) {
		if err = c.Host().RemoveAll(c.Home()); err != nil {
			return
		}
		log.Printf("%s deleted %s:%s", c, c.Host().String(), c.Home())
		c.Unload()
		return nil
	}

	log.Println(c, "must use -F or instance must be be disabled before delete")
	return nil
}
