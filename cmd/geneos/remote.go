package main

import "net/url"

// remote support

// "remote" is another component type,

// e.g.
// geneos new remote X URL
//
// below is out of date

// examples:
//
// geneos add remote name URL
//   URL here may be ssh://user@host etc.
// URL can include path to ITRS_HOME / ITRSHome, e.g.
//   ssh://user@server/home/geneos
// else it default to same as local
//
// non ssh schemes to follow
//   ssh support for agents and private key files - no passwords
//   known_hosts will be checked, no changes made, missing keys will
//   result in error. user must add hosts before use (ssh-keyscan)
//
// geneos ls remote NAME
// XXX - geneos init remote NAME
//
// remote 'localhost' is always implied
//
// geneos ls remote
// ... list remote locations
//
// geneos start gateway [name]@remote
//
// XXX support gateway pairs for standby - how ?
//
// XXX remote netprobes, auto configure with gateway for SANs etc.?
//
// support existing geneos-utils installs on remote

type Remotes struct {
	Common
	Home     string `default:"{{join .Root \"remotes\" .Name}}"`
	Hostname string
	Port     int `default:"22"`
	Username string
	ITRSHome string `default:"{{.Root}}"`
}

func init() {
	components[Remote] = ComponentFuncs{
		Instance: remoteInstance,
		Command:  nil,
		Add:      remoteAdd,
		Clean:    nil,
		Reload:   nil,
	}
}

func remoteInstance(name string) interface{} {
	// Bootstrap
	c := &Remotes{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Remote.String()
	c.Name = name
	setDefaults(&c)
	return c
}

//
// 'geneos add remote NAME SSH-URL'
//
func remoteAdd(name string, username string, params []string) (c Instance, err error) {
	if len(params) == 0 {
		log.Fatalln("remote destination must be provided in the form of a URL")
	}

	c = remoteInstance(name)

	u, err := url.Parse(params[0])
	if err != nil {
		logDebug.Println(err)
		return
	}

	switch {
	case u.Scheme == "ssh":
		if u.Host == "" {
			log.Fatalln("hostname must be provided")
		}
		setField(c, "Hostname", u.Host)
		if u.Port() != "" {
			setField(c, "Port", u.Port())
		}
		if u.User.Username() != "" {
			setField(c, "Username", u.User.Username())
		}
		if u.Path != "" {
			setField(c, "ITRSHome", u.Path)
		}
		return c, writeInstanceConfig(c)
	default:
		log.Fatalln("unsupport scheme (only ssh at the moment):", u.Scheme)
	}
	return
}
