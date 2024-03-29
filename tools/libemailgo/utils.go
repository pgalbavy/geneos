package main

import "C"
import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/go-mail/mail/v2"
)

func debug(conf EMailConfig) (debug bool) {
	if d, ok := conf["_DEBUG"]; ok {
		if strings.EqualFold(d, "true") {
			debug = true
		}
	}
	return
}

//
//
func setupMail(conf EMailConfig) (m *mail.Message, err error) {
	m = mail.NewMessage()

	var from = getWithDefault("_FROM", conf, "geneos@localhost")
	var fromName = getWithDefault("_FROM_NAME", conf, "Geneos")

	m.SetAddressHeader("From", from, fromName)

	err = addAddresses(m, conf, "To")
	if err != nil {
		return
	}

	err = addAddresses(m, conf, "Cc")
	if err != nil {
		return
	}

	err = addAddresses(m, conf, "Bcc")
	if err != nil {
		return
	}

	if !debug(conf) {
		keys := []string{"_FROM", "_FROM_NAME",
			"_TO", "_TO_NAME", "_TO_INFO_TYPE",
			"_CC", "_CC_NAME", "_CC_INFO_TYPE",
			"_BCC", "_BCC_NAME", "_BCC_INFO_TYPE",
		}
		for _, key := range keys {
			delete(conf, key)
		}
	}

	return
}

//
// Set-up a Dialer using the _SMTP_* parameters
//
// All the parameters in the official docs are supported and have the same
// defaults
//
// Additional parameters are
// * _SMTP_TLS - default / force / none (case insenstive)
// * _SMTP_USERNAME - if defined authentication attempted
// * _SMTP_PASSWORD - plain text password (overrides file below)
// * _SMTP_PASSWORD_FILE - plain text password in external file
// * XXX - _SMTP_REFERENCE - override the SMTP Reference header for conversations / threading
//
func dialServer(conf EMailConfig) (d *mail.Dialer, err error) {
	server := getWithDefault("_SMTP_SERVER", conf, "localhost")
	port := getWithDefaultInt("_SMTP_PORT", conf, 25)
	timeout := getWithDefaultInt("_SMTP_TIMEOUT", conf, 10)

	var tlsPolicy mail.StartTLSPolicy

	tls := getWithDefault("_SMTP_TLS", conf, "default")
	switch strings.ToLower(tls) {
	case "force":
		tlsPolicy = mail.MandatoryStartTLS
	case "none":
		tlsPolicy = mail.NoStartTLS
	default:
		tlsPolicy = mail.OpportunisticStartTLS
	}

	username, ok := conf["_SMTP_USERNAME"]
	if ok {
		// get the password, depending how it's configured and dial
		// _SMTP_PASSWORD_FILE first, _SMTP_PASSWORD second
		pwfile := getWithDefault("_SMTP_PASSWORD_FILE", conf, "")
		password := getWithDefault("_SMTP_PASSWORD", conf, "")
		if pwfile != "" {
			password, err = readFileString(pwfile)
			if err != nil {
				return nil, err
			}
		}
		// the password can be empty at this point. this is valid, if dumb.

		d = mail.NewDialer(server, port, username, password)
	} else {
		// no auth - initialise Dialer directly
		d = &mail.Dialer{Host: server, Port: port}
	}
	d.Timeout = time.Duration(timeout) * time.Second
	d.StartTLSPolicy = tlsPolicy

	if !debug(conf) {
		keys := []string{"_SMTP_SERVER", "_SMTP_PORT", "_SMTP_TIMEOUT",
			"_SMTP_USERNAME", "_SMTP_PASSWORD", "_SMTP_PASSWORD_FILE",
			"_SMTP_TLS",
		}
		for _, key := range keys {
			delete(conf, key)
		}
	}

	return d, nil
}

// parse the C args - "n" of them - and return a map
// a value of empty string where there is no "=" or value
func parseArgs(n C.int, args **C.char) EMailConfig {
	conf := make(EMailConfig)

	// unsafe.Slice() requires Go 1.17+
	for _, s := range unsafe.Slice(args, n) {
		t := strings.SplitN(C.GoString(s), "=", 2)
		if len(t) > 1 {
			conf[t[0]] = t[1]
		} else {
			conf[t[0]] = ""
		}
	}
	return conf
}

// return the value of the key from conf or a default
func getWithDefault(key string, conf EMailConfig, def string) string {
	if val, ok := conf[key]; ok {
		return val
	}
	return def
}

// return the value of the key from conf or a default
func getWithDefaultInt(key string, conf EMailConfig, def int) int {
	if val, ok := conf[key]; ok {
		num, err := strconv.Atoi(val)
		if err != nil {
			return 0
		}
		return num
	}
	return def
}

//
// The Geneos libemail supports an optional text name per address and also the info type,
// if given, must match "email" or "e-mail" (case insensitive). If either names or info types
// are given they MUST have the same number of members otherwise it's a fatal error
//
func addAddresses(m *mail.Message, conf EMailConfig, header string) error {
	upperHeader := strings.ToUpper(header)
	addrs := splitCommaTrimSpace(conf[fmt.Sprintf("_%s", upperHeader)])
	names := splitCommaTrimSpace(conf[fmt.Sprintf("_%s_NAME", upperHeader)])
	infotypes := splitCommaTrimSpace(conf[fmt.Sprintf("_%s_INFO_TYPE", upperHeader)])

	if len(names) > 0 && len(addrs) != len(names) {
		return fmt.Errorf("\"%s\" header items mismatch: addrs=%d != names=%d", header, len(addrs), len(names))
	}

	if len(infotypes) > 0 && len(addrs) != len(infotypes) {
		return fmt.Errorf("\"%s\" header items mismatch: addrs=%d != infotypes=%d", header, len(addrs), len(infotypes))
	}

	var addresses []string

	for i, to := range addrs {
		var name string
		if len(infotypes) > 0 {
			if !strings.EqualFold("email", infotypes[i]) && !strings.EqualFold("e-mail", infotypes[i]) {
				continue
			}
		}

		if len(names) > 0 {
			name = names[i]
		}
		addresses = append(addresses, m.FormatAddress(to, name))
	}
	m.SetHeader(header, addresses...)

	return nil
}

// split a string on commas and trim leading and trailing spaces
// an empty string results in an empty slice and NOT a slice
// with one empty value
func splitCommaTrimSpace(s string) []string {
	if s == "" {
		return []string{}
	}
	fields := strings.Split(s, ",")
	for _, field := range fields {
		strings.TrimSpace(field)
	}
	return fields
}

func readFileString(path string) (contents string, err error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return
	}
	contents = string(file)
	return
}
