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
	"io/fs"
	"os/user"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/internal/utils"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add [-t FILE] TYPE NAME",
	Short: "Add a new instance",
	Long: `Add a new instance called NAME with the TYPE supplied. The details will depends on the
	TYPE. Currently the listening port is selected automatically and other options are defaulted. If
	these need to be changed before starting, see the edit command.
	
	Gateways are given a minimal configuration file.`,
	Annotations: map[string]string{
		"wildcard": "false",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := processArgs(cmd)
		return commandAdd(ct, args, params)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().StringVarP(&addCmdTemplate, "template", "t", "", "template file to use instead of default")
	addCmd.Flags().BoolVarP(&addCmdStart, "start", "S", false, "Start new instance(s) after creation")
}

var addCmdTemplate string
var addCmdStart bool

// Add a single instance
//
// XXX argument validation is minimal
//
// remote support would be of the form name@remotename
//
func commandAdd(ct *geneos.Component, args []string, params []string) (err error) {
	var username string
	if len(args) == 0 {
		logError.Fatalln("not enough args")
	}

	// check validity and reserved words here
	name := args[0]

	_, _, rem := instance.SplitName(name, host.LOCAL)
	if err = geneos.MakeComponentDirs(rem, ct); err != nil {
		return
	}

	if utils.IsSuperuser() {
		username = viper.GetString("defaultuser")
	} else {
		u, _ := user.Current()
		username = u.Username
	}

	c, err := instance.GetInstance(ct, name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return
	}

	// check if instance already exists``
	if c.Loaded() {
		log.Println(c, "already exists")
		return
	}

	if err = c.Add(username, params, addCmdTemplate); err != nil {
		logError.Fatalln(err)
	}

	// reload config as instance data is not updated by Add() as an interface value
	c.Unload()
	c.Load()
	log.Printf("%s added, port %d\n", c, c.V().GetInt(c.Prefix("port")))

	if addCmdStart || initCmdSAN {
		instance.Start(c, nil)
		// commandStart(c.Type(), []string{name}, []string{})
	}

	return
}
