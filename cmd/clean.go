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
	geneos "wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/instance"
)

// cleanCmd represents the clean command
var cleanCmd = &cobra.Command{
	Use:   "clean [-F] [TYPE] [NAME...]",
	Short: "Clean-up instance directory",
	Long:  `Clean-up instance directories, restarting instances if doing a 'purge' clean.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("clean called")
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// cleanCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// cleanCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	cleanCmd.Flags().BoolP("purge", "F", false, "Perform a full clean. Removes more files than basic clean and restarts instances")
}

func commandClean(ct *geneos.Component, args []string, params []string) error {
	return instance.LoopCommand(ct, cleanInstance, args, params)
}

func cleanInstance(c geneos.Instance, params []string) (err error) {
	purge, err := cleanCmd.Flags().GetBool("purge")
	return instance.Clean(c, purge, params)
}
