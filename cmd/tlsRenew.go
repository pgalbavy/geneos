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
	Use:   "renew",
	Short: "Renew instance certificates",
	Long: `Renew instance certificates. All matching instances have a new
certificate issued using the current signing certificate but the
private key file is left unchanged if it exists.`,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := cmdArgsParams(cmd)
		return instance.ForAll(ct, renewInstanceCert, args, params)
	},
}

func init() {
	tlsCmd.AddCommand(tlsRenewCmd)
	tlsRenewCmd.Flags().SortFlags = false
}

// renew an instance certificate, use private key if it exists
func renewInstanceCert(c geneos.Instance, _ []string) (err error) {
	tlsDir := filepath.Join(host.Geneos(), "tls")

	hostname, _ := os.Hostname()
	if c.Host() != host.LOCAL {
		hostname = c.Host().GetString("hostname")
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
