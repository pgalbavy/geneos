package main

import (
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
)

const San Component = "san"

type SanGateway struct {
	Port   int
	Secure string
}

type Sans struct {
	InstanceBase
	BinSuffix string `default:"netprobe.linux_64"`
	SanHome   string `default:"{{join .RemoteRoot \"san\" \"sans\" .InstanceName}}"`
	SanBins   string `default:"{{join .RemoteRoot \"packages\" \"netprobe\"}}"`
	SanBase   string `default:"active_prod"`
	SanExec   string `default:"{{join .SanBins .SanBase .BinSuffix}}"`
	SanLogD   string `json:",omitempty"`
	SanLogF   string `default:"san.log"`
	SanPort   int    `default:"7036"`
	SanMode   string `json:",omitempty"`
	SanOpts   string `json:",omitempty"`
	SanLibs   string `default:"{{join .SanBins .SanBase \"lib64\"}}:{{join .SanBins .SanBase}}"`
	SanUser   string `json:",omitempty"`
	SanCert   string `json:",omitempty"`
	SanKey    string `json:",omitempty"`

	// These fields are for templating the netprobe.setup.xml file but only as placeholders
	Attributes map[string]string
	Variables  map[string]struct {
		Type  string
		Value string
	}
	Gateways map[string]SanGateway
	Types    []string
}

//go:embed templates/netprobe.setup.xml.gotmpl
var SanTemplate []byte

const SanDefaultTemplate = "netprobe.setup.xml.gotmpl"

func init() {
	RegisterComponent(Components{
		Initialise:       InitSan,
		New:              NewSan,
		ComponentType:    San,
		ParentType:       Netprobe,
		ComponentMatches: []string{"san", "sans"},
		RealComponent:    true,
		DownloadBase:     "Netprobe",
	})
	RegisterDirs([]string{
		"packages/netprobe",
		"san/sans",
		"san/templates",
	})
	RegisterSettings(GlobalSettings{
		"SanPortRange": "7036,7100-",
		"SanCleanList": "*.old",
		"SanPurgeList": "san.log:san.txt:*.snooze:*.user_assignment",
	})
}

func InitSan(remote RemoteName) {
	// copy default template to directory
	if err := writeFile(remote, GeneosPath(remote, San.String(), "templates", SanDefaultTemplate), SanTemplate, 0664); err != nil {
		log.Fatalln(err)
	}
}

func NewSan(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Sans{}
	c.RemoteRoot = GeneosRoot(remote)
	c.InstanceType = San.String()
	c.InstanceName = local
	c.InstanceLocation = remote
	setDefaults(&c)
	return c
}

// interface method set

// Return the Component for an Instance
func (n Sans) Type() Component {
	return parseComponentName(n.InstanceType)
}

func (n Sans) Name() string {
	return n.InstanceName
}

func (n Sans) Location() RemoteName {
	return n.InstanceLocation
}

func (n Sans) Home() string {
	return n.SanHome
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n Sans) Prefix(field string) string {
	return "San" + field
}

func (n Sans) Add(username string, params []string, tmpl string) (err error) {
	n.SanPort = nextPort(n.Location(), GlobalConfig["SanPortRange"])
	n.SanUser = username

	writeInstanceConfig(n)

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(&n)
	}

	return n.Rebuild()
}

// rebuild the netprobe.setup.xml file
//
// we do a dance if there is a change in TLS setup and we use default ports
func (s Sans) Rebuild() error {
	// recheck check certs/keys
	cert := getString(s, s.Prefix("Cert"))
	key := getString(s, s.Prefix("Key"))
	for gw := range s.Gateways {
		g := s.Gateways[gw]
		if cert != "" && key != "" {
			if g.Secure == "false" && g.Port == 7039 {
				g.Port = 7038
			}
			g.Secure = "true"
		} else {
			if g.Secure == "true" && g.Port == 7038 {
				g.Port = 7038
			}
			g.Secure = "false"
		}
		s.Gateways[gw] = g
	}
	writeInstanceConfig(s)
	return writeTemplate(s, filepath.Join(s.Home(), "netprobe.setup.xml"), SanDefaultTemplate)
}

func (c Sans) Command() (args, env []string) {
	logFile := getLogfilePath(c)
	args = []string{
		c.Name(),
		"-listenip", "none",
		"-port", fmt.Sprint(c.SanPort),
		"-setup", "netprobe.setup.xml",
		"-setup-interval", "300",
	}

	// add environment variables to use in setup file substitution
	env = append(env, "LOG_FILENAME="+logFile)

	if c.SanCert != "" {
		args = append(args, "-secure", "-ssl-certificate", c.SanCert)
	}

	if c.SanKey != "" {
		args = append(args, "-ssl-certificate-key", c.SanKey)
	}

	return
}

func (c Sans) Clean(purge bool, params []string) (err error) {
	logDebug.Println(c.Type(), c.Name(), "clean")
	if purge {
		var stopped bool = true
		err = stopInstance(c, params)
		if err != nil {
			if errors.Is(err, ErrProcNotExist) {
				stopped = false
			} else {
				return err
			}
		}
		if err = deletePaths(c, GlobalConfig["SanCleanList"]); err != nil {
			return err
		}
		err = deletePaths(c, GlobalConfig["SanPurgeList"])
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return deletePaths(c, GlobalConfig["SanCleanList"])
}

func (c Sans) Reload(params []string) (err error) {
	return ErrNotSupported
}
