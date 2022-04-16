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
	"wonderland.org/geneos/internal/component"
	"wonderland.org/geneos/internal/instance"
)

// reloadCmd represents the reload command
var reloadCmd = &cobra.Command{
	Use:   "reload [TYPE] [NAME...]",
	Short: "Signal the instance to reload it's configuration, if supported",
	Long:  `Signal the matching instances to reload their configurations, depending on the component TYPE.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("reload called")
	},
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}

func commandReload(ct component.ComponentType, args []string, params []string) error {
	return instance.LoopCommand(ct, reloadInstance, args, params)
}

func reloadInstance(c instance.Instance, params []string) (err error) {
	return c.Reload(params)
}
