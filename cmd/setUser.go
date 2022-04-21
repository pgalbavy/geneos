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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/geneos"
)

// setUserCmd represents the setUser command
var setUserCmd = &cobra.Command{
	Use:   "user KEY=VALUE [KEY=VALUE...]",
	Short: "Set user configuration parameters",
	Long:  ``,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := processArgs(cmd)
		return commandSetUser(ct, args, params)
	},
}

func init() {
	setCmd.AddCommand(setUserCmd)
}

func commandSetUser(ct *geneos.Component, args, params []string) error {
	userConfDir, _ := os.UserConfigDir()
	return writeConfigParams(filepath.Join(userConfDir, "geneos.json"), []string{})
}
