package main

import "C"
import (
	"log"
	"regexp"
)

// SendMail tries to duplicate the exact behaviour of libemail.so's SendMail function
// but with the addition of more modern SMTP and TLS / authentication
//
// text only, using formats, either defaults or passed in

//export SendMail
func SendMail(n C.int, args **C.char) C.int {
	conf := parseArgs(n, args)

	d, err := dialServer(conf)
	if err != nil {
		log.Println(err)
		return 1
	}

	m, err := setupMail(conf)
	if err != nil {
		log.Println(err)
		return 1
	}

	// From doc:
	// "If an _ALERT parameter is present libemail assumes it is being called as part of a gateway alert
	// and will use the appropriate format depending on the value of _ALERT_TYPE (Alert, Clear, Suspend,
	// or Resume). If no _ALERT parameter is specified libemail assumes it is being called as part of an
	// action and uses _FORMAT."
	//
	// A user defined format will always override the default format. If the _FORMAT parameter is
	// specified by the user then this will override any default formats whether or not _ALERT is present.
	//
	// Subjects behave in the same way as formats."
	//
	// Note: "ThrottleSummary" is also mentioned later, but is the same as above
	var format, subject string

	if _, ok := conf["_FORMAT"]; ok {
		format = conf["_FORMAT"]
		subject = getWithDefault("_SUBJECT", conf, defaultSubject[_SUBJECT])
	} else if _, ok = conf["_ALERT"]; ok {
		switch conf["_ALERT_TYPE"] {
		case "Alert":
			format = getWithDefault("_ALERT_FORMAT", conf, defaultFormat[_ALERT_FORMAT])
			subject = getWithDefault("_ALERT_SUBJECT", conf, defaultSubject[_ALERT_SUBJECT])
		case "Clear":
			format = getWithDefault("_CLEAR_FORMAT", conf, defaultFormat[_CLEAR_FORMAT])
			subject = getWithDefault("_CLEAR_SUBJECT", conf, defaultSubject[_CLEAR_SUBJECT])
		case "Suspend":
			format = getWithDefault("_SUSPEND_FORMAT", conf, defaultFormat[_SUSPEND_FORMAT])
			subject = getWithDefault("_SUSPEND_SUBJECT", conf, defaultSubject[_SUSPEND_SUBJECT])
		case "Resume":
			format = getWithDefault("_RESUME_FORMAT", conf, defaultFormat[_RESUME_FORMAT])
			subject = getWithDefault("_RESUME_SUBJECT", conf, defaultSubject[_RESUME_SUBJECT])
		case "ThrottleSummary":
			format = getWithDefault("_SUMMARY_FORMAT", conf, defaultFormat[_SUMMARY_FORMAT])
			subject = getWithDefault("_SUMMARY_SUBJECT", conf, defaultSubject[_SUMMARY_SUBJECT])
		default:
			format = defaultFormat[_FORMAT]
			subject = getWithDefault("_SUBJECT", conf, defaultSubject[_SUBJECT])
		}
	} else {
		format = defaultFormat[_FORMAT]
		subject = defaultSubject[_SUBJECT]
	}

	body := replArgs(format, conf)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	err = d.DialAndSend(m)
	if err != nil {
		log.Println(err)
		return 1
	}
	return 0
}

// substitue placeholder of the form %(XXX) for the value of XXX or empty and
// return the result as a new string
func replArgs(format string, conf EMailConfig) string {
	re := regexp.MustCompile(`%\([^\)]*\)`)
	result := re.ReplaceAllStringFunc(format, func(key string) string {
		// strip containing "%(...)" - as we are here, the regexp must have matched OK
		// so no further check required
		key = key[2 : len(key)-1]
		return conf[key]
	})

	return result
}
