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
	geneos "wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/internal/utils"
)

// disableCmd represents the disable command
var disableCmd = &cobra.Command{
	Use:   "disable [TYPE] [NAME...]",
	Short: "Stop and disable matching instances",
	Long:  `Mark any matching instances as disabled. The instances are also stopped.`,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := processArgs(cmd)
		return commandDisable(ct, args, params)
	},
}

func init() {
	rootCmd.AddCommand(disableCmd)
}

func commandDisable(ct *geneos.Component, args []string, params []string) (err error) {
	return instance.LoopCommand(ct, disableInstance, args, params)
}

func disableInstance(c geneos.Instance, params []string) (err error) {
	if instance.IsDisabled(c) {
		return nil
	}

	uid, gid, _, err := utils.GetUser(c.V().GetString(c.Prefix("User")))
	if err != nil {
		return
	}

	if err = instance.Stop(c, false, params); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return
	}

	disablePath := instance.ConfigPathWithExt(c, geneos.DisableExtension)

	f, err := c.Remote().Create(disablePath, 0664)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = c.Remote().Chown(disablePath, uid, gid); err != nil {
		c.Remote().Remove(disablePath)
	}

	return
}
