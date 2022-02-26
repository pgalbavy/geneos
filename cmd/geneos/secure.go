package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func init() {
	commands["secure"] = Command{
		Function:    commandSecure,
		ParseFlags:  secureFlag,
		ParseArgs:   secureArgs,
		CommandLine: "geneos secure ...",
		Summary:     `Secure options`,
		Description: ``}

	secureFlags = flag.NewFlagSet("secure", flag.ExitOnError)
	// secureFlags.BoolVar(&xxx, "x", false, "Output")
}

var secureFlags *flag.FlagSet

func secureFlag(command string, args []string) []string {
	secureFlags.Parse(args)
	checkHelpFlag(command)
	return secureFlags.Args()
}

func secureArgs(rawargs []string) (ct ComponentType, args []string, params []string) {
	if len(rawargs) == 0 {
		log.Fatalln("secure requires more arguments")
	}
	subcommand := rawargs[0]
	switch subcommand {
	case "init":
		ct = None
		args = []string{subcommand}
		return
	default:
		ct, args, params = parseArgs(rawargs[1:])
		params = append([]string{subcommand}, params...)
		return
	}
}

func commandSecure(ct ComponentType, args []string, params []string) (err error) {
	logDebug.Println(ct, args, params)
	if len(args) == 0 {
		return
	}

	if ct == None && args[0] == "init" {
		return secureInit()
	}

	return loopCommand(secureInstance, ct, args, params)
}

func secureInstance(c Instance, params []string) (err error) {
	log.Println("run for", Type(c), Name(c))
	if len(params) == 0 {
		log.Fatalln("you must supply an action to take")
	}
	switch params[0] {
	case "new":
		// create a cert, overwrite any existing
		// re-user private key if it exists
		return createInstanceCert(c)
	case "ls":
		// show certs and expiries
	}
	return
}

// create the tls/ directory in ITRSHome and a CA / DCA as required
//
// later options to allow import of a DCA
func secureInit() (err error) {
	tlsPath := filepath.Join(RunningConfig.ITRSHome, "tls")
	// directory permissions do not need to be restrictive
	err = os.MkdirAll(tlsPath, 0777)
	if err != nil {
		log.Fatalln(err)
	}

	err = createRootCA(tlsPath)
	if err != nil {
		log.Fatalln(err)
	}

	err = createIntrCA(tlsPath)
	if err != nil {
		log.Fatalln(err)
	}

	return
}

func createRootCA(dir string) (err error) {
	// create rootCA.pem / rootCA.key
	rootCertPath := filepath.Join(dir, "rootCA.pem")
	rootKeyPath := filepath.Join(dir, "rootCA.key")

	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
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

	cert, rootKey, err := generateCertAndKey(&template, &template, nil)
	if err != nil {
		log.Fatalln(err)
	}

	err = writeCert(cert, rootCertPath)
	if err != nil {
		log.Fatalln(err)
	}
	err = writeKey(rootKey, rootKeyPath)
	if err != nil {
		log.Fatalln(err)
	}
	return
}

func createIntrCA(dir string) (err error) {
	intrCertPath := filepath.Join(dir, "intrCA.pem")
	intrKeyPath := filepath.Join(dir, "intrCA.key")

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "geneos intermediate CA",
		},
		NotBefore:             time.Now().Add(-60 * time.Second),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		//ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		MaxPathLen: 1,
	}

	rootCert, err := readCert(filepath.Join(dir, "rootCA.pem"))
	if err != nil {
		log.Fatalln(err)
	}
	rootKey, err := readKey(filepath.Join(dir, "rootCA.key"))
	if err != nil {
		log.Fatalln(err)
	}

	cert, intrKey, err := generateCertAndKey(&template, rootCert, rootKey)
	if err != nil {
		log.Fatalln(err)
	}

	err = writeCert(cert, intrCertPath)
	if err != nil {
		log.Fatalln(err)
	}
	err = writeKey(intrKey, intrKeyPath)
	if err != nil {
		log.Fatalln(err)
	}

	return
}

func createInstanceCert(c Instance) (err error) {
	tlsDir := filepath.Join(RunningConfig.ITRSHome, "tls")

	host, _ := os.Hostname()
	template := x509.Certificate{
		SerialNumber: big.NewInt(int64(time.Now().Nanosecond())),
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("geneos %s %s certificate", Type(c), Name(c)),
		},
		NotBefore:      time.Now().Add(-60 * time.Second),
		NotAfter:       time.Now().AddDate(1, 0, 0),
		KeyUsage:       x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		MaxPathLenZero: true,
		DNSNames:       []string{"localhost", host},
		IPAddresses:    []net.IP{net.ParseIP("127.0.0.1")},
	}

	intrCert, err := readCert(filepath.Join(tlsDir, "intrCA.pem"))
	if err != nil {
		log.Fatalln(err)
	}
	intrKey, err := readKey(filepath.Join(tlsDir, "intrCA.key"))
	if err != nil {
		log.Fatalln(err)
	}

	cert, key, err := generateCertAndKey(&template, intrCert, intrKey)
	if err != nil {
		log.Fatalln(err)
	}

	err = saveCert(c, cert)
	if err != nil {
		log.Fatalln(err)
	}
	err = saveKey(c, key)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("certificate created for", Type(c), Name(c))
	return
}

func writeCert(cert *x509.Certificate, path string) (err error) {
	logDebug.Println("write cert to", path)
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
	err = os.WriteFile(path, certPEM, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	return

}

func saveCert(c Instance, cert *x509.Certificate) (err error) {
	if c == nil || Type(c) == None {
		log.Fatalln(err)
	}

	return writeCert(cert, getString(c, Prefix(c)+"Cert"))
}

func writeKey(key *rsa.PrivateKey, path string) (err error) {
	logDebug.Println("write key to", path)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	err = os.WriteFile(path, keyPEM, 0400)
	if err != nil {
		log.Fatalln(err)
	}
	return
}

func saveKey(c Instance, key *rsa.PrivateKey) (err error) {
	if Type(c) == None {
		log.Fatalln(err)
	}

	return writeKey(key, getString(c, Prefix(c)+"Key"))
}

func readCert(path string) (cert *x509.Certificate, err error) {
	certPEM, err := os.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}

	p, _ := pem.Decode(certPEM)
	if p.Type != "CERTIFICATE" {
		log.Fatalln("not a cert")
	}
	return x509.ParseCertificate(p.Bytes)
}

func loadCert(c Instance) (cert *x509.Certificate, err error) {
	if Type(c) == None {
		log.Fatalln(err)
	}

	return readCert(filepath.Join(Home(c), getString(Prefix(c), "Cert")))
}

func readKey(path string) (key *rsa.PrivateKey, err error) {
	keyPEM, err := os.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}

	p, _ := pem.Decode(keyPEM)
	if p.Type != "RSA PRIVATE KEY" {
		log.Fatalln("not a private key")
	}
	return x509.ParsePKCS1PrivateKey(p.Bytes)
}

func loadKey(c Instance) (key *rsa.PrivateKey, err error) {
	if Type(c) == None {
		log.Fatalln(err)
	}

	return readKey(filepath.Join(Home(c), getString(Prefix(c), "Key")))
}

func generateCertAndKey(template, parent *x509.Certificate, parentKey *rsa.PrivateKey) (cert *x509.Certificate, key *rsa.PrivateKey, err error) {
	key, err = rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		log.Fatalln(err)
	}

	privKey := key
	if parentKey != nil {
		privKey = parentKey
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, privKey)
	if err != nil {
		log.Fatalln(err)
	}

	cert, err = x509.ParseCertificate(certBytes)
	if err != nil {
		log.Fatalln(err)
	}

	return
}
