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
	"wonderland.org/geneos/internal/instance"
)

// tlsSyncCmd represents the tlsSync command
var tlsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync remote hosts certificate chain files",
	Long: `Create a chain.pem file made up of the root and signing
certificates and then copy them to all remote hosts. This can
then be used to verify connections from components.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("tlsSync called")
	},
}

func init() {
	tlsCmd.AddCommand(tlsSyncCmd)
}

// if there is a local tls/chain.pem file then copy it to all remotes
// overwriting any existing versions
func TLSSync() (err error) {
	rootCert, _ := instance.ReadRootCert()
	geneosCert, _ := instance.ReadSigningCert()

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
		if err = r.WriteCerts(filepath.Join(tlsPath, "chain.pem"), rootCert, geneosCert); err != nil {
			return
		}

		log.Println("Updated chain.pem on", r.String())
	}
	return
}
