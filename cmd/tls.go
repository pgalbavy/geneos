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
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/component"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
)

// tlsCmd represents the tls command
var tlsCmd = &cobra.Command{
	Use:   "tls",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("tls called")
	},
	Hidden: true,
}

func init() {
	rootCmd.AddCommand(tlsCmd)
}

const rootCAFile = "rootCA"
const signingCertFile = "geneos"

func matchKey(cert *x509.Certificate, keys []*rsa.PrivateKey) (index int, err error) {
	for i, key := range keys {
		if key.PublicKey.Equal(cert.PublicKey) {
			return i, nil
		}
	}
	return -1, os.ErrNotExist
}

func writeInstanceCert(c instance.Instance, cert *x509.Certificate) (err error) {
	if c.Type() == component.None {
		return ErrInvalidArgs
	}
	certfile := c.Type().String() + ".pem"
	if err = c.Remote().writeCert(filepath.Join(c.Home(), certfile), cert); err != nil {
		return
	}
	if c.V.GetString(c.Prefix("Cert")) == certfile {
		return
	}
	if err = setField(c, c.Prefix("Cert"), certfile); err != nil {
		return
	}

	return writeInstanceConfig(c)
}

func writeInstanceKey(c instance.Instance, key *rsa.PrivateKey) (err error) {
	if c.Type() == component.None {
		return ErrInvalidArgs
	}

	keyfile := c.Type().String() + ".key"
	if err = writeKey(c.Remote(), filepath.Join(c.Home(), keyfile), key); err != nil {
		return
	}
	if c.V.GetString(c.Prefix("Key")) == keyfile {
		return
	}
	c.V.Set(c.Prefix("Key"), keyfile)
	return writeInstanceConfig(c)
}

// write a private key as PEM to path. sets file permissions to 0600 (before umask)
func writeKey(r *host.Host, path string, key *rsa.PrivateKey) (err error) {
	logDebug.Println("write key to", path)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return r.WriteFile(path, keyPEM, 0600)
}

// write cert as PEM to path
func writeCert(r *host.Host, path string, cert *x509.Certificate) (err error) {
	logDebug.Println("write cert to", path)
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})

	return r.WriteFile(path, certPEM, 0644)
}

// concatenate certs and write to path
func writeCerts(r *host.Host, path string, certs ...*x509.Certificate) (err error) {
	logDebug.Println("write certs to", path)
	var certsPEM []byte
	for _, cert := range certs {
		p := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
		certsPEM = append(certsPEM, p...)
	}
	return r.WriteFile(path, certsPEM, 0644)
}

// read a PEM encoded cert from path, return the first found as a parsed certificate
func readCert(r *host.Host, path string) (cert *x509.Certificate, err error) {
	certPEM, err := r.ReadFile(path)
	if err != nil {
		return
	}

	for {
		p, rest := pem.Decode(certPEM)
		if p == nil {
			return nil, fmt.Errorf("cannot locate certificate in %s", path)
		}
		if p.Type == "CERTIFICATE" {
			return x509.ParseCertificate(p.Bytes)
		}
		certPEM = rest
	}
}

// read the rootCA certificate from the installation directory
func readRootCert() (cert *x509.Certificate, err error) {
	tlsDir := filepath.Join(Geneos(), "tls")
	return readCert(host.LOCAL, filepath.Join(tlsDir, rootCAFile+".pem"))
}

// read the signing certificate from the installation directory
func readSigningCert() (cert *x509.Certificate, err error) {
	tlsDir := filepath.Join(Geneos(), "tls")
	return readCert(host.LOCAL, filepath.Join(tlsDir, signingCertFile+".pem"))
}

// read the instance certificate
func readInstanceCert(c instance.Instance) (cert *x509.Certificate, err error) {
	if c.Type() == component.None {
		return nil, ErrInvalidArgs
	}

	if c.V.GetString(c.Prefix("Cert")) == "" {
		return nil, os.ErrNotExist
	}
	return readCert(c.Remote(), instanceAbsPath(c, c.V.GetString(c.Prefix("Cert"))))
}

// read a PEM encoded RSA private key from path. returns the first found as
// a parsed key
func readKey(r *host.Host, path string) (key *rsa.PrivateKey, err error) {
	keyPEM, err := r.ReadFile(path)
	if err != nil {
		return
	}

	for {
		p, rest := pem.Decode(keyPEM)
		if p == nil {
			return nil, fmt.Errorf("cannot locate RSA private key in %s", path)
		}
		if p.Type == "RSA PRIVATE KEY" {
			return x509.ParsePKCS1PrivateKey(p.Bytes)
		}
		keyPEM = rest
	}
}

// read the instance RSA private key
func readInstanceKey(c instance.Instance) (key *rsa.PrivateKey, err error) {
	if c.Type() == component.None {
		return nil, ErrInvalidArgs
	}

	return readKey(c.Remote(), instanceAbsPath(c, c.V.GetString(c.Prefix("Key"))))
}

// wrapper to create a new certificate given the sign cert and private key and an optional private key to (re)use
// for the created certificate itself. returns a certificate and private key
func createCert(template, parent *x509.Certificate, parentKey *rsa.PrivateKey, existingKey *rsa.PrivateKey) (cert *x509.Certificate, key *rsa.PrivateKey, err error) {
	if existingKey != nil {
		key = existingKey
	} else {
		key, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return
		}
	}

	privKey := key
	if parentKey != nil {
		privKey = parentKey
	}

	var certBytes []byte
	if certBytes, err = x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, privKey); err == nil {
		cert, err = x509.ParseCertificate(certBytes)
	}

	return
}
