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
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
)

// tlsRenewCmd represents the tlsRenew command
var tlsRenewCmd = &cobra.Command{
	Use:   "tlsRenew",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("tlsRenew called")
	},
}

func init() {
	tlsCmd.AddCommand(tlsRenewCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tlsRenewCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tlsRenewCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// renew an instance certificate, use private key if it exists
func renewInstanceCert(c geneos.Instance) (err error) {
	tlsDir := filepath.Join(Geneos(), "tls")

	hostname, _ := os.Hostname()
	if c.Remote() != host.LOCAL {
		hostname = c.Remote().V().GetString("Hostname")
	}

	serial, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		return
	}
	expires := time.Now().AddDate(1, 0, 0)
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("geneos %s %s", c.Type(), c.Name()),
		},
		NotBefore:      time.Now().Add(-60 * time.Second),
		NotAfter:       expires,
		KeyUsage:       x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		MaxPathLenZero: true,
		DNSNames:       []string{hostname},
		// IPAddresses:    []net.IP{net.ParseIP("127.0.0.1")},
	}

	intrCert, err := host.LOCAL.ReadCert(filepath.Join(tlsDir, geneos.SigningCertFile+".pem"))
	if err != nil {
		return
	}
	intrKey, err := host.LOCAL.ReadKey(filepath.Join(tlsDir, geneos.SigningCertFile+".key"))
	if err != nil {
		return
	}

	// read existing key or create a new one
	existingKey, _ := instance.ReadKey(c)
	cert, key, err := instance.CreateCertKey(&template, intrCert, intrKey, existingKey)
	if err != nil {
		return
	}

	if err = instance.WriteCert(c, cert); err != nil {
		return
	}

	if existingKey == nil {
		if err = instance.WriteKey(c, key); err != nil {
			return
		}
	}

	log.Printf("certificate renewed for %s (expires %s)", c, expires)

	return
}
