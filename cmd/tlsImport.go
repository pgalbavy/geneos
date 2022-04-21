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
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
)

// tlsImportCmd represents the tlsImport command
var tlsImportCmd = &cobra.Command{
	Use:   "tlsImport",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Annotations: map[string]string{
		"wildcard": "false",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, _, params := processArgs(cmd)
		return TLSImport(params...)
	},
}

func init() {
	tlsCmd.AddCommand(tlsImportCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tlsImportCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tlsImportCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// import root and signing certs
//
// a root cert is one where subject == issuer
//
// no support for instance certs (yet)
func TLSImport(sources ...string) (err error) {
	logDebug.Println(sources)
	tlsPath := filepath.Join(Geneos(), "tls")
	err = host.LOCAL.MkdirAll(tlsPath, 0755)
	if err != nil {
		return
	}

	// save certs and keys into memory, then check certs for root / etc.
	// and then validate private keys against certs before saving
	// anything to disk
	var certs []*x509.Certificate
	var keys []*rsa.PrivateKey
	var f []byte

	for _, source := range sources {
		logDebug.Println("importing", source)
		if f, err = geneos.ReadLocalFileOrURL(source); err != nil {
			logError.Println(err)
			err = nil
			continue
		}

		for {
			block, rest := pem.Decode(f)
			if block == nil {
				break
			}
			switch block.Type {
			case "CERTIFICATE":
				cert, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					return err
				}
				certs = append(certs, cert)
			case "RSA PRIVATE KEY":
				key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
				if err != nil {
					return err
				}
				if err = key.Validate(); err != nil {
					return err
				}
				keys = append(keys, key)
			default:
				return fmt.Errorf("unknown PEM type found: %s", block.Type)
			}
			f = rest
		}
	}

	var title, prefix string
	for _, cert := range certs {
		if bytes.Equal(cert.RawSubject, cert.RawIssuer) {
			// root cert
			title = "root"
			prefix = geneos.RootCAFile
		} else {
			// signing cert
			title = "signing"
			prefix = geneos.SigningCertFile
		}
		i, err := matchKey(cert, keys)
		if err != nil {
			logDebug.Println("cert: no matching key found, ignoring", cert.Subject.String())
			continue
		}

		// pull out the matching key, write files
		key := keys[i]
		if len(keys) > i {
			keys = append(keys[:i], keys[i+1:]...)
		} else {
			keys = keys[:i]
		}

		if err = host.LOCAL.WriteCert(filepath.Join(tlsPath, prefix+".pem"), cert); err != nil {
			return err
		}
		log.Printf("imported %s certificate to %q", title, filepath.Join(tlsPath, prefix+".pem"))
		if err = host.LOCAL.WriteKey(filepath.Join(tlsPath, prefix+".key"), key); err != nil {
			return err
		}
		log.Printf("imported %s RSA private key to %q", title, filepath.Join(tlsPath, prefix+".pem"))
	}

	return
}

func matchKey(cert *x509.Certificate, keys []*rsa.PrivateKey) (index int, err error) {
	for i, key := range keys {
		if key.PublicKey.Equal(cert.PublicKey) {
			return i, nil
		}
	}
	return -1, os.ErrNotExist
}
