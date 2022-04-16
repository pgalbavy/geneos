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

// tlsInitCmd represents the tlsInit command
var tlsInitCmd = &cobra.Command{
	Use:   "tlsInit",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("tlsInit called")
	},
}

func init() {
	tlsCmd.AddCommand(tlsInitCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tlsInitCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tlsInitCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// create the tls/ directory in Geneos and a CA / DCA as required
//
// later options to allow import of a DCA
func TLSInit() (err error) {
	tlsPath := filepath.Join(Geneos(), "tls")
	// directory permissions do not need to be restrictive
	err = host.LOCAL.MkdirAll(tlsPath, 0775)
	if err != nil {
		logError.Fatalln(err)
	}

	rootCert, err := newRootCA(tlsPath)
	if err != nil {
		logError.Fatalln(err)
	}

	interCert, err := newIntrCA(tlsPath)
	if err != nil {
		logError.Fatalln(err)
	}

	// concatenate a chain
	if err = writeCerts(host.LOCAL, filepath.Join(tlsPath, "chain.pem"), rootCert, interCert); err != nil {
		logError.Fatalln(err)
	}
	log.Println("created chain.pem")

	return TLSSync()
}
