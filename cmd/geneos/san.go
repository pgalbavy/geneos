package main

import (
	_ "embed"
	"errors"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/sftp"
)

const San Component = "san"

type Sans struct {
	InstanceBase
	BinSuffix string `default:"netprobe.linux_64"`
	SanHome   string `default:"{{join .InstanceRoot \"san\" \"sans\" .InstanceName}}"`
	SanBins   string `default:"{{join .InstanceRoot \"packages\" \"netprobe\"}}"`
	SanBase   string `default:"active_prod"`
	SanExec   string `default:"{{join .SanBins .SanBase .BinSuffix}}"`
	SanLogD   string `default:"{{.SanHome}}"`
	SanLogF   string `default:"san.log"`
	SanPort   int    `default:"7036"`
	SanMode   string `json:",omitempty"`
	SanOpts   string `json:",omitempty"`
	SanLibs   string `default:"{{join .SanBins .SanBase \"lib64\"}}:{{join .SanBins .SanBase}}"`
	SanUser   string `json:",omitempty"`
	SanCert   string `json:",omitempty"`
	SanKey    string `json:",omitempty"`
}

//go:embed netprobe.setup.Template.xml
var emptySANTemplate string

func init() {
	RegisterComponent(Components{
		New:              NewSan,
		ComponentType:    San,
		ComponentMatches: []string{"san", "sans"},
		IncludeInLoops:   true,
		DownloadBase:     "Netprobe",
	})
	RegisterDirs([]string{
		"packages/netprobe",
		"san/sans",
	})
	RegisterSettings(GlobalSettings{
		"SanPortRange": "7036,7100-",
		"SanCleanList": "*.old",
		"SanPurgeList": "san.log:san.txt:*.snooze:*.user_assignment",
	})
}

func NewSan(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Sans{}
	c.InstanceRoot = remoteRoot(remote)
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

func (n Sans) Location() string {
	return n.InstanceLocation
}

func (n Sans) Home() string {
	return n.SanHome
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n Sans) Prefix(field string) string {
	return "San" + field
}

func (n Sans) Create(username string, params []string) (err error) {
	n.SanPort = nextPort(n.Location(), GlobalConfig["SanPortRange"])
	n.SanUser = username

	writeInstanceConfig(n)

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(n)
	}

	// default config XML etc.
	t, err := template.New("empty").Funcs(textJoinFuncs).Parse(emptySANTemplate)
	if err != nil {
		logError.Fatalln(err)
	}

	var out io.Writer

	switch n.Location() {
	case LOCAL:
		var cf *os.File
		cf, err = os.Create(filepath.Join(n.Home(), "netprobe.setup.xml"))
		out = cf
		if err != nil {
			log.Println(err)
			return
		}
		defer cf.Close()
		if err = cf.Chmod(0664); err != nil {
			logError.Fatalln(err)
		}
	default:
		var cf *sftp.File
		cf, err = createRemoteFile(n.Location(), filepath.Join(n.Home(), "netprobe.setup.xml"))
		out = cf
		if err != nil {
			log.Println(err)
			return
		}
		defer cf.Close()
		if err = cf.Chmod(0664); err != nil {
			logError.Fatalln(err)
		}
	}

	if err = t.Execute(out, n); err != nil {
		logError.Fatalln(err)
	}

	return nil
}

func (c Sans) Command() (args, env []string) {
	logFile := getLogfilePath(c)
	args = []string{
		c.Name(),
		"-listenip", "none",
		"-setup", "netprobe.setup.xml",
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
		if err = removePathList(c, GlobalConfig["SanCleanList"]); err != nil {
			return err
		}
		err = removePathList(c, GlobalConfig["SanPurgeList"])
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return removePathList(c, GlobalConfig["SanCleanList"])
}

func (c Sans) Reload(params []string) (err error) {
	return ErrNotSupported
}
