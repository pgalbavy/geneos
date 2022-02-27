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

const rootCAFile = "rootCA"
const intermediateFile = "geneos"

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
	case "init", "import":
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

	if ct == None && args[0] == "import" {
		return secureImport(args)
	}

	return loopCommand(secureInstance, ct, args, params)
}

func secureInstance(c Instance, params []string) (err error) {
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

	rootCert, err := createRootCA(tlsPath)
	if err != nil {
		log.Fatalln(err)
	}

	interCert, err := createIntrCA(tlsPath)
	if err != nil {
		log.Fatalln(err)
	}

	// concatenate a chain
	writeCerts(filepath.Join(tlsPath, "chain.pem"), rootCert, interCert)

	return
}

// import intermediate (signing) cert and key from files on command line
// loop through args and decode pem, check type and import - filename to
// be decided (CN.pem etc.)
func secureImport(files []string) (err error) {
	tlsPath := filepath.Join(RunningConfig.ITRSHome, "tls")
	for _, source := range files {
		f, err := readSource(source)
		if err != nil {
			log.Fatalln(err)
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
					log.Fatalln(err)
				}
				writeCert(filepath.Join(tlsPath, intermediateFile+".pem"), cert)
			case "RSA PRIVATE KEY":
				key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
				if err != nil {
					log.Fatalln(err)
				}
				writeKey(filepath.Join(tlsPath, intermediateFile+".key"), key)
			default:
				log.Fatalln("unknown PEM type:", block.Type)
			}

			f = rest
		}
	}
	return
}

func createRootCA(dir string) (cert *x509.Certificate, err error) {
	// create rootCA.pem / rootCA.key
	rootCertPath := filepath.Join(dir, rootCAFile+".pem")
	rootKeyPath := filepath.Join(dir, rootCAFile+".key")

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

	cert, key, err := createCert(&template, &template, nil, nil)
	if err != nil {
		log.Fatalln(err)
	}

	err = writeCert(rootCertPath, cert)
	if err != nil {
		log.Fatalln(err)
	}
	err = writeKey(rootKeyPath, key)
	if err != nil {
		log.Fatalln(err)
	}
	return
}

func createIntrCA(dir string) (cert *x509.Certificate, err error) {
	intrCertPath := filepath.Join(dir, intermediateFile+".pem")
	intrKeyPath := filepath.Join(dir, intermediateFile+".key")

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
		MaxPathLen:            1,
	}

	rootCert, err := readCert(filepath.Join(dir, rootCAFile+".pem"))
	if err != nil {
		log.Fatalln(err)
	}
	rootKey, err := readKey(filepath.Join(dir, rootCAFile+".key"))
	if err != nil {
		log.Fatalln(err)
	}

	cert, key, err := createCert(&template, rootCert, rootKey, nil)
	if err != nil {
		log.Fatalln(err)
	}

	err = writeCert(intrCertPath, cert)
	if err != nil {
		log.Fatalln(err)
	}
	err = writeKey(intrKeyPath, key)
	if err != nil {
		log.Fatalln(err)
	}

	return
}

func createInstanceCert(c Instance) (err error) {
	tlsDir := filepath.Join(RunningConfig.ITRSHome, "tls")

	host, _ := os.Hostname()
	serial, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		log.Fatalln(err)
	}
	template := x509.Certificate{
		SerialNumber: serial,
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

	intrCert, err := readCert(filepath.Join(tlsDir, intermediateFile+".pem"))
	if err != nil {
		log.Fatalln(err)
	}
	intrKey, err := readKey(filepath.Join(tlsDir, intermediateFile+".key"))
	if err != nil {
		log.Fatalln(err)
	}

	existingKey, _ := readInstanceKey(c)
	cert, key, err := createCert(&template, intrCert, intrKey, existingKey)
	if err != nil {
		log.Fatalln(err)
	}

	err = writeInstanceCert(c, cert)
	if err != nil {
		log.Fatalln(err)
	}

	if existingKey == nil {
		err = writeInstanceKey(c, key)
		if err != nil {
			log.Fatalln(err)
		}
	}

	log.Println("certificate created for", Type(c), Name(c))
	return
}

func writeCert(path string, cert *x509.Certificate) (err error) {
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

func writeCerts(path string, certs ...*x509.Certificate) (err error) {
	logDebug.Println("write certs to", path)
	var certsPEM []byte
	for _, cert := range certs {
		p := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
		certsPEM = append(certsPEM, p...)
	}
	err = os.WriteFile(path, certsPEM, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	return
}

func writeInstanceCert(c Instance, cert *x509.Certificate) (err error) {
	if c == nil || Type(c) == None {
		log.Fatalln(err)
	}

	return writeCert(getString(c, Prefix(c)+"Cert"), cert)
}

func writeKey(path string, key *rsa.PrivateKey) (err error) {
	logDebug.Println("write key to", path)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	err = os.WriteFile(path, keyPEM, 0640)
	if err != nil {
		log.Fatalln(err)
	}
	return
}

func writeInstanceKey(c Instance, key *rsa.PrivateKey) (err error) {
	if Type(c) == None {
		log.Fatalln(err)
	}

	return writeKey(getString(c, Prefix(c)+"Key"), key)
}

func readCert(path string) (cert *x509.Certificate, err error) {
	certPEM, err := os.ReadFile(path)
	if err != nil {
		return
	}

	p, _ := pem.Decode(certPEM)
	if p.Type != "CERTIFICATE" {
		err = fmt.Errorf("not a cert")
		return
	}
	return x509.ParseCertificate(p.Bytes)
}

func readInstanceCert(c Instance) (cert *x509.Certificate, err error) {
	if Type(c) == None {
		log.Fatalln(err)
	}

	return readCert(getString(c, Prefix(c)+"Cert"))
}

func readKey(path string) (key *rsa.PrivateKey, err error) {
	keyPEM, err := os.ReadFile(path)
	if err != nil {
		return
	}

	p, _ := pem.Decode(keyPEM)
	if p.Type != "RSA PRIVATE KEY" {
		err = fmt.Errorf("not a private key")
		return
	}
	return x509.ParsePKCS1PrivateKey(p.Bytes)
}

func readInstanceKey(c Instance) (key *rsa.PrivateKey, err error) {
	if Type(c) == None {
		log.Fatalln(err)
	}

	return readKey(getString(c, Prefix(c)+"Key"))
}

func createCert(template, parent *x509.Certificate, parentKey *rsa.PrivateKey, existingKey *rsa.PrivateKey) (cert *x509.Certificate, key *rsa.PrivateKey, err error) {
	if existingKey != nil {
		key = existingKey
	} else {
		key, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			log.Fatalln(err)
		}
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
