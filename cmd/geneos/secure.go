package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/csv"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"
)

func init() {
	commands["secure"] = Command{
		Function:    commandSecure,
		ParseFlags:  flagsSecure,
		ParseArgs:   secureArgs,
		CommandLine: "geneos secure ...",
		Summary:     `Secure options`,
		Description: ``}

	secureFlags = flag.NewFlagSet("secure", flag.ExitOnError)
	// support the same flags as "ls" for lists
	secureFlags.BoolVar(&listJSON, "j", false, "Output JSON")
	secureFlags.BoolVar(&listJSONIndent, "i", false, "Indent / pretty print JSON")
	secureFlags.BoolVar(&listCSV, "c", false, "Output CSV")
	secureFlags.BoolVar(&helpFlag, "h", false, helpUsage)
}

var secureFlags *flag.FlagSet

const rootCAFile = "rootCA"
const intermediateFile = "geneos"

// skip over subcommand, which is required
func flagsSecure(command string, args []string) (ret []string) {
	if len(args) == 0 {
		return
	}
	secureFlags.Parse(args[1:])
	checkHelpFlag(command)
	return append([]string{args[0]}, secureFlags.Args()...)
}

// pop subcommand, parse args, put subcommand back onto params?
func secureArgs(rawargs []string) (ct ComponentType, args []string, params []string) {
	if len(rawargs) == 0 {
		log.Fatalln("command requires more arguments - help text here")
	}
	subcommand := rawargs[0]
	ct, args, params = parseArgs(rawargs[1:])
	args = append([]string{subcommand}, args...)
	return
}

func commandSecure(ct ComponentType, args []string, params []string) (err error) {
	logDebug.Println(ct, args, params)

	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case "init":
		if err = secureInit(); err != nil {
			log.Fatalln(err)
		}
		return loopSubcommand(secureInstance, "new", ct, args, params)
	case "import":
		return secureImport(args)
	case "ls":
		return listCertsCommand(ct, args, params)
	}

	return loopSubcommand(secureInstance, subcommand, ct, args, params)
}

type lsCertType struct {
	Type        string
	Name        string
	Remaining   time.Duration
	Expires     time.Time
	CommonName  string
	Issuer      string
	SubAltNames []string
	IPs         []net.IP
}

func listCertsCommand(ct ComponentType, args []string, params []string) (err error) {
	tlsDir := filepath.Join(RunningConfig.ITRSHome, "tls")
	rootCert, err2 := readCert(filepath.Join(tlsDir, rootCAFile+".pem"))
	if err2 != nil {
		return err2
	}
	geneosCert, err2 := readCert(filepath.Join(tlsDir, intermediateFile+".pem"))
	if err2 != nil {
		return err2
	}

	switch {
	case listJSON:
		jsonEncoder = json.NewEncoder(log.Writer())
		if listJSONIndent {
			jsonEncoder.SetIndent("", "    ")
		}
		jsonEncoder.Encode(lsCertType{
			"global",
			rootCAFile,
			time.Duration(time.Until(rootCert.NotAfter).Seconds()),
			rootCert.NotAfter,
			rootCert.Subject.CommonName,
			rootCert.Issuer.CommonName,
			nil,
			nil,
		})
		jsonEncoder.Encode(lsCertType{
			"global",
			intermediateFile,
			time.Duration(time.Until(geneosCert.NotAfter).Seconds()),
			geneosCert.NotAfter,
			geneosCert.Subject.CommonName,
			geneosCert.Issuer.CommonName,
			nil,
			nil,
		})
		err = loopCommand(lsInstanceCertJSON, ct, args, params)
	case listCSV:
		csvWriter = csv.NewWriter(log.Writer())
		csvWriter.Write([]string{
			"Type",
			"Name",
			"Remaining",
			"Expires",
			"CommonName",
			"Issuer",
			"SubjAltNames",
			"IPs",
		})
		csvWriter.Write([]string{
			"global",
			rootCAFile,
			fmt.Sprintf("%v", time.Until(rootCert.NotAfter).Seconds()),
			rootCert.NotAfter.String(),
			rootCert.Subject.CommonName,
			rootCert.Issuer.CommonName,
			"[]",
			"[]",
		})
		csvWriter.Write([]string{
			"global",
			intermediateFile,
			fmt.Sprintf("%v", time.Until(geneosCert.NotAfter).Seconds()),
			geneosCert.NotAfter.String(),
			geneosCert.Subject.CommonName,
			geneosCert.Issuer.CommonName,
			"[]",
			"[]",
		})
		err = loopCommand(lsInstanceCertCSV, ct, args, params)
		csvWriter.Flush()
	default:
		lsTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
		fmt.Fprintf(lsTabWriter, "Type\tName\tRemaining\tExpires\tCommonName\tIssuer\tSubjAltNames\tIPs\n")
		fmt.Fprintf(lsTabWriter, "global\t%s\t%.f\t%q\t%q\t%q\t\t\t\n", rootCAFile,
			time.Until(rootCert.NotAfter).Seconds(), rootCert.NotAfter,
			rootCert.Subject.CommonName, rootCert.Issuer.CommonName)
		fmt.Fprintf(lsTabWriter, "global\t%s\t%.f\t%q\t%q\t%q\t\t\t\n", intermediateFile,
			time.Until(geneosCert.NotAfter).Seconds(), geneosCert.NotAfter,
			geneosCert.Subject.CommonName, geneosCert.Issuer.CommonName)
		err = loopCommand(lsInstanceCert, ct, args, params)
		lsTabWriter.Flush()
	}
	return
}

func secureInstance(c Instance, subcommand string, params []string) (err error) {
	logDebug.Println("secureInstance:", Type(c), Name(c), subcommand, params)
	switch subcommand {
	case "new":
		// create a cert, DO NOT overwrite any existing unless renewing
		// re-user private key if it exists
		return createInstanceCert(c)
	case "renew":
		return renewInstanceCert(c)
	}
	return
}

func lsInstanceCert(c Instance, params []string) (err error) {
	cert, err := readInstanceCert(c)
	if err != nil {
		return
	}
	expires := cert.NotAfter
	fmt.Fprintf(lsTabWriter, "%s\t%s\t%.f\t%q\t%q\t%q\t", Type(c), Name(c), time.Until(expires).Seconds(), expires, cert.Subject.CommonName, cert.Issuer.CommonName)
	if len(cert.DNSNames) > 0 {
		fmt.Fprintf(lsTabWriter, "%v", cert.DNSNames)
	}
	fmt.Fprintf(lsTabWriter, "\t")
	if len(cert.IPAddresses) > 0 {
		fmt.Fprintf(lsTabWriter, "%v", cert.IPAddresses)
	}
	fmt.Fprintf(lsTabWriter, "\n")
	return
}

func lsInstanceCertCSV(c Instance, params []string) (err error) {
	cert, err := readInstanceCert(c)
	if err != nil {
		return
	}
	expires := cert.NotAfter
	until := fmt.Sprintf("%0f", time.Until(expires).Seconds())
	cols := []string{Type(c).String(), Name(c), until, expires.String(), cert.Subject.CommonName, cert.Issuer.CommonName}
	cols = append(cols, fmt.Sprintf("%v", cert.DNSNames))
	cols = append(cols, fmt.Sprintf("%v", cert.IPAddresses))

	csvWriter.Write(cols)
	return
}

func lsInstanceCertJSON(c Instance, params []string) (err error) {
	cert, err := readInstanceCert(c)
	if err != nil {
		return
	}
	jsonEncoder.Encode(lsCertType{Type(c).String(), Name(c), time.Duration(time.Until(cert.NotAfter).Seconds()),
		cert.NotAfter, cert.Subject.CommonName, cert.Issuer.CommonName, cert.DNSNames, cert.IPAddresses})
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

	rootCert, err := newRootCA(tlsPath)
	if err != nil {
		log.Fatalln(err)
	}

	interCert, err := newIntrCA(tlsPath)
	if err != nil {
		log.Fatalln(err)
	}

	// concatenate a chain
	if err = writeCerts(filepath.Join(tlsPath, "chain.pem"), rootCert, interCert); err != nil {
		log.Fatalln(err)
	}
	log.Println("created chain.pem")

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

func newRootCA(dir string) (cert *x509.Certificate, err error) {
	// create rootCA.pem / rootCA.key
	rootCertPath := filepath.Join(dir, rootCAFile+".pem")
	rootKeyPath := filepath.Join(dir, rootCAFile+".key")

	if cert, err = readCert(rootCertPath); err == nil {
		log.Println(rootCAFile, "already exists")
		return
	}
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
	log.Println("CA certificate created for", rootCAFile)

	return
}

func newIntrCA(dir string) (cert *x509.Certificate, err error) {
	intrCertPath := filepath.Join(dir, intermediateFile+".pem")
	intrKeyPath := filepath.Join(dir, intermediateFile+".key")

	if cert, err = readCert(intrCertPath); err == nil {
		log.Println(intermediateFile, "already exists")
		return
	}

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

	log.Println("CA certificate created for", intermediateFile)

	return
}

// renew an instance certificate, leave the private key untouched
//
// if private key doesn't exist, do we error?
func renewInstanceCert(c Instance) (err error) {
	tlsDir := filepath.Join(RunningConfig.ITRSHome, "tls")

	host, _ := os.Hostname()
	serial, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		log.Fatalln(err)
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("geneos %s %s", Type(c), Name(c)),
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

	log.Println("certificate renewed for", Type(c), Name(c))
	return
}

// create a new certificate for an instance
//
// this also creates a new private key
//
// skip if certificate exists (no expiry check)
func createInstanceCert(c Instance) (err error) {
	tlsDir := filepath.Join(RunningConfig.ITRSHome, "tls")

	// skip if we can load an existing certificate
	if _, err = readInstanceCert(c); err == nil {
		return
	}

	host, _ := os.Hostname()
	serial, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		log.Fatalln(err)
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("geneos %s %s", Type(c), Name(c)),
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

	cert, key, err := createCert(&template, intrCert, intrKey, nil)
	if err != nil {
		log.Fatalln(err)
	}

	err = writeInstanceCert(c, cert)
	if err != nil {
		log.Fatalln(err)
	}

	err = writeInstanceKey(c, key)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("certificate created for", Type(c), Name(c))
	return
}

func writeInstanceCert(c Instance, cert *x509.Certificate) (err error) {
	if c == nil || Type(c) == None {
		log.Fatalln(err)
	}
	certfile := Type(c).String() + ".pem"
	if err = writeCert(filepath.Join(Home(c), certfile), cert); err != nil {
		return
	}
	if err = setField(c, Prefix(c)+"Cert", certfile); err != nil {
		return
	}
	return writeInstanceConfig(c)
}

func writeInstanceKey(c Instance, key *rsa.PrivateKey) (err error) {
	if Type(c) == None {
		log.Fatalln(err)
	}

	keyfile := Type(c).String() + ".key"
	if err = writeKey(filepath.Join(Home(c), keyfile), key); err != nil {
		return
	}
	if err = setField(c, Prefix(c)+"Key", keyfile); err != nil {
		return
	}
	return writeInstanceConfig(c)
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

	return readCert(filepathForInstance(c, getString(c, Prefix(c)+"Cert")))
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

	return readKey(filepathForInstance(c, getString(c, Prefix(c)+"Key")))
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
