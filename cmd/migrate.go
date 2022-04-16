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
	"io/fs"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/component"
	"wonderland.org/geneos/internal/instance"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate [TYPE] [NAME...]",
	Short: "Migrate legacy .rc configuration to .json",
	Long: `Migrate any legacy .rc configuration files to JSON format and
	rename the .rc file to .rc.orig.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("migrate called")
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// migrateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// migrateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func commandMigrate(ct component.ComponentType, names []string, params []string) (err error) {
	return instance.LoopCommand(ct, migrateInstance, names, params)
}

func migrateInstance(c instance.Instance, params []string) (err error) {
	if err = migrateConfig(c); err != nil {
		log.Println(c, "cannot migrate configuration", err)
	}
	return
}

// migrate config from .rc to .json, but check first
func migrateConfig(c instance.Instance) (err error) {
	// if no .rc, return
	if _, err = c.Remote().Stat(instance.ConfigPathWithExt(c, "rc")); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	// if .json exists, return
	if _, err = c.Remote().Stat(instance.ConfigPathWithExt(c, "json")); err == nil {
		return nil
	}

	// write new .json
	if err = writeInstanceConfig(c); err != nil {
		logError.Println("failed to write config file:", err)
		return
	}

	// back-up .rc
	if err = c.Remote().Rename(instance.ConfigPathWithExt(c, "rc"), instance.ConfigPathWithExt(c, "rc.orig")); err != nil {
		logError.Println("failed to rename old config:", err)
	}

	logDebug.Printf("migrated %s to JSON config", c)
	return
}
