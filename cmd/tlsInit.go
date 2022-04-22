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
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
)

// tlsInitCmd represents the tlsInit command
var tlsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise the TLS environment",
	Long: `Initialise the TLS environment by creating a self-signed
root certificate to act as a CA and a signing certificate signed
by the root. Any instances will have certificates created for
them but configurations will not be rebuilt.`,
	Annotations: map[string]string{
		"wildcard": "false",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		// _, _, params := processArgs(cmd)
		return TLSInit()
	},
}

func init() {
	tlsCmd.AddCommand(tlsInitCmd)
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
	if err = host.LOCAL.WriteCerts(filepath.Join(tlsPath, "chain.pem"), rootCert, interCert); err != nil {
		logError.Fatalln(err)
	}
	log.Println("created chain.pem")

	return TLSSync()
}

func newRootCA(dir string) (cert *x509.Certificate, err error) {
	// create rootCA.pem / rootCA.key
	rootCertPath := filepath.Join(dir, geneos.RootCAFile+".pem")
	rootKeyPath := filepath.Join(dir, geneos.RootCAFile+".key")

	if cert, err = instance.ReadRootCert(); err == nil {
		log.Println(geneos.RootCAFile, "already exists")
		return
	}
	serial, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "geneos root CA",
		},
		NotBefore:             time.Now().Add(-60 * time.Second),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		MaxPathLen:            2,
	}

	cert, key, err := instance.CreateCertKey(&template, &template, nil, nil)
	if err != nil {
		return
	}

	if err = host.LOCAL.WriteCert(rootCertPath, cert); err != nil {
		return
	}
	if err = host.LOCAL.WriteKey(rootKeyPath, key); err != nil {
		return
	}
	log.Println("CA certificate created for", geneos.RootCAFile)

	return
}

func newIntrCA(dir string) (cert *x509.Certificate, err error) {
	intrCertPath := filepath.Join(dir, geneos.SigningCertFile+".pem")
	intrKeyPath := filepath.Join(dir, geneos.SigningCertFile+".key")

	if cert, err = instance.ReadSigningCert(); err == nil {
		log.Println(geneos.SigningCertFile, "already exists")
		return
	}

	serial, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		return
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "geneos intermediate CA",
		},
		NotBefore:             time.Now().Add(-60 * time.Second),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		MaxPathLen:            1,
	}

	rootCert, err := instance.ReadRootCert()
	if err != nil {
		return
	}
	rootKey, err := host.LOCAL.ReadKey(filepath.Join(dir, geneos.RootCAFile+".key"))
	if err != nil {
		return
	}

	cert, key, err := instance.CreateCertKey(&template, rootCert, rootKey, nil)
	if err != nil {
		return
	}

	if err = host.LOCAL.WriteCert(intrCertPath, cert); err != nil {
		return
	}
	if err = host.LOCAL.WriteKey(intrKeyPath, key); err != nil {
		return
	}

	log.Println("Signing certificate created for", geneos.SigningCertFile)

	return
}
