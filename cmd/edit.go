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
	"os/exec"

	"github.com/spf13/cobra"
	geneos "wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/internal/utils"
)

// editCmd represents the edit command
var editCmd = &cobra.Command{
	Use:   "edit [TYPE] [NAME...]",
	Short: "Open an editor for instance configuration file",
	Long: `Open an editor for JSON configuration file(s). If the literal 'global' or 'user' is supplied then the respective configuration file is opened, otherwise one or more configuration files are opened, depending on if TYPE and NAME(s) are supplied. The text editor invoked will be the first set of the environment variables VISUAL or EDITOR or the linux
	/usr/bin/editor alternative will be used. e.g.
	
		VISUAL=code geneos edit user
	
	will open a VS Code editor window for the user configuration file.`,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := processArgs(cmd)
		return commandEdit(ct, args, params)
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}

//
// run the configured editor against the instance chosen
//
func commandEdit(ct *geneos.Component, args []string, params []string) (err error) {
	// default for no args is to edit user config
	if len(args) == 0 {
		args = []string{"user"}
	}

	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			// let the Linux alternatives system sort it out
			editor = "editor"
		}
	}

	// instance config files ?
	if utils.IsSuperuser() {
		logError.Fatalln("no editing instance configs as root, for now")
	}

	// loop instances - parse the args again and load/print the config,
	// XXX allow for RC files again
	var cs []string
	for _, name := range args {
		for _, c := range instance.FindInstances(ct, name) {
			if c.Remote() != host.LOCAL {
				logError.Println("remote edit of", c, ErrNotSupported)
				continue
			}
			if _, err = host.LOCAL.Stat(instance.ConfigPathWithExt(c, "rc")); err == nil {
				cs = append(cs, instance.ConfigPathWithExt(c, "rc"))
			} else if _, err = c.Remote().Stat(instance.ConfigPathWithExt(c, "json")); err == nil {
				cs = append(cs, instance.ConfigPathWithExt(c, "json"))
			} else {
				logError.Println("no configuration file found for", c)
				continue
			}
		}
	}
	if len(cs) > 0 {
		err = editConfigFiles(editor, cs...)
	}

	return
}

func editConfigFiles(editor string, files ...string) (err error) {
	cmd := exec.Command(editor, files...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
