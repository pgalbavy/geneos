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
	commands["tls"] = Command{
		Function:    commandTLS,
		ParseFlags:  flagsTLS,
		ParseArgs:   TLSArgs,
		CommandLine: "geneos tls [init|import|new|renew|ls] ...",
		Summary:     `TLS operations`,
		Description: `TLS operations. The following subcommands are supported:

	geneos tls init
		initialise the TLS environment, creating root and intermediate signing CAs and certificates for all instances

	geneos tls import file [file...]
		import certificate and private key used to sign instance certificates

	geneos tls new [TYPE] [NAME]
		create a new certificate for matching instances

	geneos tls renew [TYPE] [NAME]
		renew certificates for matching instances
		
	geneos tls ls [TYPE] [NAME]
		list certificates for matcing instances, including the root and intermediate CA certs.
		same options as for the main 'ls' command

	geneos tls sync
		copy the current chain.pem to all known remotes
		this is also done by 'init' if remotes are configured at that point
`}

	TLSFlags = flag.NewFlagSet("tls", flag.ExitOnError)
	// support the same flags as "ls" for lists
	TLSFlags.BoolVar(&TLSlistJSON, "j", false, "Output JSON")
	TLSFlags.BoolVar(&TLSlistLong, "l", false, "Long output")
	TLSFlags.BoolVar(&TLSlistJSONIndent, "i", false, "Indent / pretty print JSON")
	TLSFlags.BoolVar(&TLSlistCSV, "c", false, "Output CSV")
	TLSFlags.BoolVar(&helpFlag, "h", false, helpUsage)
}

var TLSFlags *flag.FlagSet
var TLSlistJSON, TLSlistJSONIndent, TLSlistLong bool
var TLSlistCSV bool

const rootCAFile = "rootCA"
const signingCertFile = "geneos"

// skip over subcommand, which is required
func flagsTLS(command string, args []string) (ret []string) {
	if len(args) == 0 {
		return
	}
	TLSFlags.Parse(args[1:])
	checkHelpFlag(command)
	return append([]string{args[0]}, TLSFlags.Args()...)
}

// pop subcommand, parse args, put subcommand back onto params?
func TLSArgs(rawargs []string) (ct Component, args []string, params []string) {
	if len(rawargs) == 0 {
		logError.Fatalln("command requires more arguments - help text here")
	}
	subcommand := rawargs[0]
	ct, args, params = defaultArgs(rawargs[1:])
	args = append([]string{subcommand}, args...)
	return
}

func commandTLS(ct Component, args []string, params []string) (err error) {
	logDebug.Println(ct, args, params)

	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case "init":
		if err = TLSInit(); err != nil {
			logError.Fatalln(err)
		}
		return loopSubcommand(TLSInstance, "new", ct, args, params)
	case "import":
		return TLSImport(params)
	case "ls":
		return listCertsCommand(ct, args, params)
	case "sync":
		return TLSSync()
	}

	return loopSubcommand(TLSInstance, subcommand, ct, args, params)
}

type lsCertType struct {
	Type        string
	Name        string
	Location    string
	Remaining   time.Duration
	Expires     time.Time
	CommonName  string
	Issuer      string
	SubAltNames []string
	IPs         []net.IP
}

func listCertsCommand(ct Component, args []string, params []string) (err error) {
	rootCert, _ := readRootCert()
	geneosCert, _ := readSigningCert()

	switch {
	case TLSlistJSON:
		jsonEncoder = json.NewEncoder(log.Writer())
		if TLSlistJSONIndent {
			jsonEncoder.SetIndent("", "    ")
		}
		if rootCert != nil {
			jsonEncoder.Encode(lsCertType{
				"global",
				rootCAFile,
				LOCAL,
				time.Duration(time.Until(rootCert.NotAfter).Seconds()),
				rootCert.NotAfter,
				rootCert.Subject.CommonName,
				rootCert.Issuer.CommonName,
				nil,
				nil,
			})
		}
		if geneosCert != nil {
			jsonEncoder.Encode(lsCertType{
				"global",
				signingCertFile,
				LOCAL,
				time.Duration(time.Until(geneosCert.NotAfter).Seconds()),
				geneosCert.NotAfter,
				geneosCert.Subject.CommonName,
				geneosCert.Issuer.CommonName,
				nil,
				nil,
			})
		}
		err = loopCommand(lsInstanceCertJSON, ct, args, params)
	case TLSlistCSV:
		csvWriter = csv.NewWriter(log.Writer())
		csvWriter.Write([]string{
			"Type",
			"Name",
			"Location",
			"Remaining",
			"Expires",
			"CommonName",
			"Issuer",
			"SubjAltNames",
			"IPs",
		})
		if rootCert != nil {
			csvWriter.Write([]string{
				"global",
				rootCAFile,
				LOCAL,
				fmt.Sprintf("%0f", time.Until(rootCert.NotAfter).Seconds()),
				rootCert.NotAfter.String(),
				rootCert.Subject.CommonName,
				rootCert.Issuer.CommonName,
				"[]",
				"[]",
			})
		}
		if geneosCert != nil {
			csvWriter.Write([]string{
				"global",
				signingCertFile,
				LOCAL,
				fmt.Sprintf("%0f", time.Until(geneosCert.NotAfter).Seconds()),
				geneosCert.NotAfter.String(),
				geneosCert.Subject.CommonName,
				geneosCert.Issuer.CommonName,
				"[]",
				"[]",
			})
		}
		err = loopCommand(lsInstanceCertCSV, ct, args, params)
		csvWriter.Flush()
	default:
		lsTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
		fmt.Fprintf(lsTabWriter, "Type\tName\tLocation\tRemaining\tExpires\tCommonName\tIssuer\tSubjAltNames\tIPs\n")
		if rootCert != nil {
			fmt.Fprintf(lsTabWriter, "global\t%s\t%s\t%.f\t%q\t%q\t%q\t\t\t\n", rootCAFile, LOCAL,
				time.Until(rootCert.NotAfter).Seconds(), rootCert.NotAfter,
				rootCert.Subject.CommonName, rootCert.Issuer.CommonName)
		}
		if geneosCert != nil {
			fmt.Fprintf(lsTabWriter, "global\t%s\t%s\t%.f\t%q\t%q\t%q\t\t\t\n", signingCertFile, LOCAL,
				time.Until(geneosCert.NotAfter).Seconds(), geneosCert.NotAfter,
				geneosCert.Subject.CommonName, geneosCert.Issuer.CommonName)
		}
		err = loopCommand(lsInstanceCert, ct, args, params)
		lsTabWriter.Flush()
	}
	return
}

func TLSInstance(c Instances, subcommand string, params []string) (err error) {
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

func lsInstanceCert(c Instances, params []string) (err error) {
	cert, err := readInstanceCert(c)
	if err == ErrNotFound {
		// this is OK - readInstanceCert() reports no configured cert this way
		return nil
	}
	if err != nil {
		return
	}
	expires := cert.NotAfter
	fmt.Fprintf(lsTabWriter, "%s\t%s\t%s\t%.f\t%q\t%q\t%q\t", c.Type(), c.Name(), c.Location(), time.Until(expires).Seconds(), expires, cert.Subject.CommonName, cert.Issuer.CommonName)
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

func lsInstanceCertCSV(c Instances, params []string) (err error) {
	cert, err := readInstanceCert(c)
	if err == ErrNotFound {
		// this is OK
		return nil
	}
	if err != nil {
		return
	}
	expires := cert.NotAfter
	until := fmt.Sprintf("%0f", time.Until(expires).Seconds())
	cols := []string{c.Type().String(), c.Name(), c.Location(), until, expires.String(), cert.Subject.CommonName, cert.Issuer.CommonName}
	cols = append(cols, fmt.Sprintf("%v", cert.DNSNames))
	cols = append(cols, fmt.Sprintf("%v", cert.IPAddresses))

	csvWriter.Write(cols)
	return
}

func lsInstanceCertJSON(c Instances, params []string) (err error) {
	cert, err := readInstanceCert(c)
	if err == ErrNotFound {
		// this is OK
		return nil
	}
	if err != nil {
		return
	}
	jsonEncoder.Encode(lsCertType{c.Type().String(), c.Name(), c.Location(), time.Duration(time.Until(cert.NotAfter).Seconds()),
		cert.NotAfter, cert.Subject.CommonName, cert.Issuer.CommonName, cert.DNSNames, cert.IPAddresses})
	return
}

// create the tls/ directory in ITRSHome and a CA / DCA as required
//
// later options to allow import of a DCA
func TLSInit() (err error) {
	tlsPath := filepath.Join(ITRSHome(), "tls")
	// directory permissions do not need to be restrictive
	err = mkdirAll(LOCAL, tlsPath, 0777)
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
	if err = writeCerts(LOCAL, filepath.Join(tlsPath, "chain.pem"), rootCert, interCert); err != nil {
		logError.Fatalln(err)
	}
	log.Println("created chain.pem")

	return TLSSync()
}

// if there is a local tls/chain.pem file then copy it to all remotes
// overwriting any existing versions
func TLSSync() (err error) {
	rootCert, _ := readRootCert()
	geneosCert, _ := readSigningCert()

	if rootCert == nil && geneosCert == nil {
		return
	}

	for _, remote := range allRemotes() {
		if remote.Name() == LOCAL {
			continue
		}
		tlsPath := filepath.Join(remoteRoot(remote.Name()), "tls")
		if err = mkdirAll(remote.Name(), tlsPath, 0775); err != nil {
			logError.Fatalln(err)
		}
		if err = writeCerts(remote.Name(), filepath.Join(tlsPath, "chain.pem"), rootCert, geneosCert); err != nil {
			logError.Fatalln(err)
		}
		log.Println("Updated chain.pem on", remote.Name())
	}
	return
}

// import intermediate (signing) cert and key from files on command line
// loop through args and decode pem, check type and import - filename to
// be decided (CN.pem etc.)
func TLSImport(files []string) (err error) {
	tlsPath := filepath.Join(ITRSHome(), "tls")
	err = mkdirAll(LOCAL, tlsPath, 0755)
	if err != nil {
		log.Fatalln(err)
	}
	for _, source := range files {
		f := readSourceBytes(source)
		if len(f) == 0 {
			logError.Fatalln("read faile:", source)
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
					logError.Fatalln(err)
				}
				if err = writeCert(LOCAL, filepath.Join(tlsPath, signingCertFile+".pem"), cert); err != nil {
					log.Fatalln(err)
				}
				log.Printf("imported certificate %q to %q", source, filepath.Join(tlsPath, signingCertFile+".pem"))
			case "RSA PRIVATE KEY":
				key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
				if err != nil {
					logError.Fatalln(err)
				}
				if err = writeKey(LOCAL, filepath.Join(tlsPath, signingCertFile+".key"), key); err != nil {
					log.Fatalln(err)
				}
				log.Printf("imported RSA private key %q to %q", source, filepath.Join(tlsPath, signingCertFile+".pem"))
			default:
				logError.Fatalln("unknown PEM type:", block.Type)
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

	if cert, err = readRootCert(); err == nil {
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
		logError.Fatalln(err)
	}

	err = writeCert(LOCAL, rootCertPath, cert)
	if err != nil {
		logError.Fatalln(err)
	}
	err = writeKey(LOCAL, rootKeyPath, key)
	if err != nil {
		logError.Fatalln(err)
	}
	log.Println("CA certificate created for", rootCAFile)

	return
}

func newIntrCA(dir string) (cert *x509.Certificate, err error) {
	intrCertPath := filepath.Join(dir, signingCertFile+".pem")
	intrKeyPath := filepath.Join(dir, signingCertFile+".key")

	if cert, err = readSigningCert(); err == nil {
		log.Println(signingCertFile, "already exists")
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

	rootCert, err := readRootCert()
	if err != nil {
		logError.Fatalln(err)
	}
	rootKey, err := readKey(LOCAL, filepath.Join(dir, rootCAFile+".key"))
	if err != nil {
		logError.Fatalln(err)
	}

	cert, key, err := createCert(&template, rootCert, rootKey, nil)
	if err != nil {
		logError.Fatalln(err)
	}

	err = writeCert(LOCAL, intrCertPath, cert)
	if err != nil {
		logError.Fatalln(err)
	}
	err = writeKey(LOCAL, intrKeyPath, key)
	if err != nil {
		logError.Fatalln(err)
	}

	log.Println("CA certificate created for", signingCertFile)

	return
}

// create a new certificate for an instance
//
// this also creates a new private key
//
// skip if certificate exists (no expiry check)
func createInstanceCert(c Instances) (err error) {
	tlsDir := filepath.Join(ITRSHome(), "tls")

	// skip if we can load an existing certificate
	if _, err = readInstanceCert(c); err == nil {
		return
	}

	host, _ := os.Hostname()
	if c.Location() != LOCAL {
		r := loadRemoteConfig(c.Location())
		host = getString(r, "Hostname")
	}

	serial, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		logError.Fatalln(err)
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
		DNSNames:       []string{host},
		// IPAddresses:    []net.IP{net.ParseIP("127.0.0.1")},
	}

	intrCert, err := readSigningCert()
	if err != nil {
		logError.Fatalln(err)
	}
	intrKey, err := readKey(LOCAL, filepath.Join(tlsDir, signingCertFile+".key"))
	if err != nil {
		logError.Fatalln(err)
	}

	cert, key, err := createCert(&template, intrCert, intrKey, nil)
	if err != nil {
		logError.Fatalln(err)
	}

	err = writeInstanceCert(c, cert)
	if err != nil {
		logError.Fatalln(err)
	}

	err = writeInstanceKey(c, key)
	if err != nil {
		logError.Fatalln(err)
	}

	log.Printf("certificate created for %s %s@%s (expires %s)", c.Type(), c.Name(), c.Location(), expires)

	return
}

// renew an instance certificate, leave the private key untouched
//
// if private key doesn't exist, do we error?
func renewInstanceCert(c Instances) (err error) {
	tlsDir := filepath.Join(ITRSHome(), "tls")

	host, _ := os.Hostname()
	if c.Location() != LOCAL {
		r := loadRemoteConfig(c.Location())
		host = getString(r, "Hostname")
	}

	serial, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		logError.Fatalln(err)
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
		DNSNames:       []string{host},
		// IPAddresses:    []net.IP{net.ParseIP("127.0.0.1")},
	}

	intrCert, err := readCert(LOCAL, filepath.Join(tlsDir, signingCertFile+".pem"))
	if err != nil {
		logError.Fatalln(err)
	}
	intrKey, err := readKey(LOCAL, filepath.Join(tlsDir, signingCertFile+".key"))
	if err != nil {
		logError.Fatalln(err)
	}

	existingKey, err := readInstanceKey(c)
	if err != nil {
		logError.Fatalln(err)
	}
	cert, key, err := createCert(&template, intrCert, intrKey, existingKey)
	if err != nil {
		logError.Fatalln(err)
	}

	err = writeInstanceCert(c, cert)
	if err != nil {
		logError.Fatalln(err)
	}

	if existingKey == nil {
		err = writeInstanceKey(c, key)
		if err != nil {
			logError.Fatalln(err)
		}
	}

	log.Printf("certificate renewed for %s %s@%s (expires %s)", c.Type(), c.Name(), c.Location(), expires)

	return
}

func writeInstanceCert(c Instances, cert *x509.Certificate) (err error) {
	if c.Type() == None {
		logError.Fatalln(err)
	}
	certfile := c.Type().String() + ".pem"
	if err = writeCert(c.Location(), filepath.Join(c.Home(), certfile), cert); err != nil {
		return
	}
	if err = setField(c, c.Prefix("Cert"), certfile); err != nil {
		return
	}

	if err = writeInstanceConfig(c); err != nil {
		log.Fatalln(err)
	}
	return
}

func writeInstanceKey(c Instances, key *rsa.PrivateKey) (err error) {
	if c.Type() == None {
		logError.Fatalln(err)
	}

	keyfile := c.Type().String() + ".key"
	if err = writeKey(c.Location(), filepath.Join(c.Home(), keyfile), key); err != nil {
		return
	}
	if err = setField(c, c.Prefix("Key"), keyfile); err != nil {
		return
	}
	return writeInstanceConfig(c)
}

func writeKey(remote, path string, key *rsa.PrivateKey) (err error) {
	logDebug.Println("write key to", path)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	err = writeFile(remote, path, keyPEM, 0640)
	if err != nil {
		logError.Fatalln(err)
	}
	return
}

func writeCert(remote, path string, cert *x509.Certificate) (err error) {
	logDebug.Println("write cert to", path)
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})

	err = writeFile(remote, path, certPEM, 0644)

	if err != nil {
		logError.Fatalln(err)
	}
	return
}

func writeCerts(remote string, path string, certs ...*x509.Certificate) (err error) {
	logDebug.Println("write certs to", path)
	var certsPEM []byte
	for _, cert := range certs {
		p := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
		certsPEM = append(certsPEM, p...)
	}
	err = writeFile(remote, path, certsPEM, 0644)
	if err != nil {
		logError.Fatalln(err)
	}
	return
}

func readCert(remote string, path string) (cert *x509.Certificate, err error) {
	certPEM, err := readFile(remote, path)
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

func readRootCert() (cert *x509.Certificate, err error) {
	tlsDir := filepath.Join(ITRSHome(), "tls")
	return readCert(LOCAL, filepath.Join(tlsDir, rootCAFile+".pem"))
}

func readSigningCert() (cert *x509.Certificate, err error) {
	tlsDir := filepath.Join(ITRSHome(), "tls")
	return readCert(LOCAL, filepath.Join(tlsDir, signingCertFile+".pem"))
}

func readInstanceCert(c Instances) (cert *x509.Certificate, err error) {
	if c.Type() == None {
		logError.Fatalln(err)
	}

	if getString(c, c.Prefix("Cert")) == "" {
		return nil, ErrNotFound
	}
	return readCert(c.Location(), filepathForInstance(c, getString(c, c.Prefix("Cert"))))
}

func readKey(remote string, path string) (key *rsa.PrivateKey, err error) {
	keyPEM, err := readFile(remote, path)
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

func readInstanceKey(c Instances) (key *rsa.PrivateKey, err error) {
	if c.Type() == None {
		logError.Fatalln(err)
	}

	return readKey(c.Location(), filepathForInstance(c, getString(c, c.Prefix("Key"))))
}

func createCert(template, parent *x509.Certificate, parentKey *rsa.PrivateKey, existingKey *rsa.PrivateKey) (cert *x509.Certificate, key *rsa.PrivateKey, err error) {
	if existingKey != nil {
		key = existingKey
	} else {
		key, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			logError.Fatalln(err)
		}
	}

	privKey := key
	if parentKey != nil {
		privKey = parentKey
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, privKey)
	if err != nil {
		logError.Fatalln(err)
	}

	cert, err = x509.ParseCertificate(certBytes)
	if err != nil {
		logError.Fatalln(err)
	}

	return
}
