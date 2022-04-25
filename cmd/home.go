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
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
)

// homeCmd represents the home command
var homeCmd = &cobra.Command{
	Use:   "home [TYPE] [NAME]",
	Short: "Output the home directory of the installation or the first matching instance",
	Long: `Output the home directory of the first matching instance or local
	installation or the remote on stdout. This is intended for scripting,
	e.g.
	
		cd $(geneos home)
		cd $(geneos home gateway example1)
			
	Because of the intended use no errors are logged and no other output.
	An error in the examples above result in the user's home
	directory being selected.`,
	SilenceUsage: true,
	Annotations: map[string]string{
		"wildcard": "false",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := processArgs(cmd)
		return commandHome(ct, args, params)
	},
}

func init() {
	rootCmd.AddCommand(homeCmd)
}

func commandHome(_ *geneos.Component, args []string, params []string) error {
	var ct *geneos.Component
	if len(args) == 0 {
		log.Println(host.Geneos())
		return nil
	}

	// check if first arg is a type, if not set to None else pop first arg
	if ct = geneos.ParseComponentName(args[0]); ct == nil {
		ct = nil
	} else {
		args = args[1:]
	}

	var i []geneos.Instance
	if len(args) == 0 {
		i = instance.GetAll(host.LOCAL, ct)
	} else {
		i = instance.MatchAll(ct, args[0])
	}

	if len(i) == 0 {
		log.Println(host.Geneos())
		return nil
	}

	log.Println(i[0].Home())
	return nil
}
