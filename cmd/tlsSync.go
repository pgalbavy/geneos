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
	"path/filepath"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/host"
)

// tlsSyncCmd represents the tlsSync command
var tlsSyncCmd = &cobra.Command{
	Use:   "tlsSync",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("tlsSync called")
	},
}

func init() {
	tlsCmd.AddCommand(tlsSyncCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tlsSyncCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tlsSyncCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// if there is a local tls/chain.pem file then copy it to all remotes
// overwriting any existing versions
func TLSSync() (err error) {
	rootCert, _ := readRootCert()
	geneosCert, _ := readSigningCert()

	if rootCert == nil && geneosCert == nil {
		return
	}

	for _, r := range host.AllHosts() {
		if r == host.LOCAL {
			continue
		}
		tlsPath := r.GeneosPath("tls")
		if err = r.MkdirAll(tlsPath, 0775); err != nil {
			return
		}
		if err = writeCerts(r, filepath.Join(tlsPath, "chain.pem"), rootCert, geneosCert); err != nil {
			return
		}

		log.Println("Updated chain.pem on", r.String())
	}
	return
}
