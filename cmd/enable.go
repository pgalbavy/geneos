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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/component"
	"wonderland.org/geneos/internal/instance"
)

// enableCmd represents the enable command
var enableCmd = &cobra.Command{
	Use:   "enable [-S] [TYPE] [NAME...]",
	Short: "Enable one or more instances. Only previously disabled instances are started",
	Long:  `Mark any matching instances as enabled and if this changes status then start the instance.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("enable called")
	},
}

func init() {
	rootCmd.AddCommand(enableCmd)

	enableCmd.Flags().BoolVarP(&enableCmdStart, "start", "S", false, "Start enabled instances")
}

var enableCmdStart bool

// simpler than disable, just try to remove the flag file
// we do also start the component(s)
func commandEneable(ct component.ComponentType, args []string, params []string) (err error) {
	return instance.LoopCommand(ct, enableInstance, args, params)
}

func enableInstance(c instance.Instance, params []string) (err error) {
	err = c.Remote().Remove(instance.ConfigPathWithExt(c, disableExtension))
	if (err == nil || errors.Is(err, os.ErrNotExist)) && enableCmdStart {
		startInstance(c, params)
	}
	return nil
}
