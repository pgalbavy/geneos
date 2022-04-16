package san

import (
	_ "embed"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/component"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/internal/instance/fa2"
	"wonderland.org/geneos/internal/instance/netprobe"
)

const San component.ComponentType = "san"

type Sans struct {
	instance.Instance
	// The SanType is used to select the base netprobe type - either Netprobe or FA2
	SanType string

	BinSuffix string `default:"{{if eq .SanType \"fa2\"}}fix-analyser2-{{end}}netprobe.linux_64"`
	SanHome   string `default:"{{join .RemoteRoot \"san\" \"sans\" .InstanceName}}"`
	SanBins   string `default:"{{join .RemoteRoot \"packages\" .SanType}}"`
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

	// The SAN configuration name may be diffrent to the instance name
	SanName string `default:"{{.InstanceName}}"`

	// These fields are for templating the netprobe.setup.xml file but only as placeholders
	Attributes map[string]string
	Variables  map[string]string // key = name, value = type:value (names must be unique)
	Gateways   map[string]int
	Types      []string
}

//go:embed templates/netprobe.setup.xml.gotmpl
var SanTemplate []byte

const SanDefaultTemplate = "netprobe.setup.xml.gotmpl"

func init() {
	component.RegisterComponent(component.Components{
		Initialise:       InitSan,
		New:              NewSan,
		ComponentType:    San,
		RelatedTypes:     []component.ComponentType{netprobe.Netprobe, fa2.FA2},
		ComponentMatches: []string{"san", "sans"},
		RealComponent:    true,
		DownloadBase:     "Netprobe",
		PortRange:        "SanPortRange",
		CleanList:        "SanCleanList",
		PurgeList:        "SanPurgeList",
		DefaultSettings: map[string]string{
			"SanPortRange": "7036,7100-",
			"SanCleanList": "*.old",
			"SanPurgeList": "san.log:san.txt:*.snooze:*.user_assignment",
		},
	})
	San.RegisterDirs([]string{
		"packages/netprobe",
		"san/sans",
		"san/templates",
	})
}

func InitSan(r *host.Host) {
	// copy default template to directory
	if err := component.MakeComponentDirs(r, San); err != nil {
		logError.Fatalln(err)
	}
	if err := r.WriteFile(r.GeneosPath(San.String(), "templates", SanDefaultTemplate), SanTemplate, 0664); err != nil {
		logError.Fatalln(err)
	}
}

var sans sync.Map

func NewSan(name string) interface{} {
	ct, local, r := instance.SplitInstanceName(name, host.LOCAL)
	s, ok := sans.Load(r.FullName(local))
	if ok {
		sn, ok := s.(*Sans)
		if ok {
			return sn
		}
	}
	c := &Sans{}
	c.V = viper.New()
	c.InstanceRemote = r
	c.RemoteRoot = r.GeneosRoot()
	c.InstanceType = San.String()
	c.InstanceName = local
	c.SanType = string(netprobe.Netprobe)
	if ct != component.None {
		c.SanType = string(ct)
	}
	if err := setDefaults(&c); err != nil {
		logError.Fatalln(c, "setDefaults():", err)
	}
	c.InstanceLocation = host.Name(r.String())
	sans.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (s *Sans) Type() component.ComponentType {
	return component.ParseComponentName(s.InstanceType)
}

func (s *Sans) Name() string {
	return s.InstanceName
}

func (s *Sans) Location() host.Name {
	return s.InstanceLocation
}

func (s *Sans) Home() string {
	return s.SanHome
}

// Prefix() takes the string argument and adds any component type specific prefix
func (s *Sans) Prefix(field string) string {
	return "San" + field
}

func (s *Sans) Remote() *host.Host {
	return s.InstanceRemote
}

func (s *Sans) Base() *instance.Instance {
	return &s.Instance
}

func (s *Sans) String() string {
	return s.Type().String() + ":" + s.InstanceName + "@" + s.Location().String()
}

func (s *Sans) Load() (err error) {
	if s.ConfigLoaded {
		return
	}
	err = loadConfig(s)
	s.ConfigLoaded = err == nil
	return
}

func (s *Sans) Unload() (err error) {
	sans.Delete(s.Name() + "@" + s.Location().String())
	s.ConfigLoaded = false
	return
}

func (s *Sans) Loaded() bool {
	return s.ConfigLoaded
}

func (s *Sans) Add(username string, params []string, tmpl string) (err error) {
	s.SanPort = instance.NextPort(s.InstanceRemote, San)
	s.SanUser = username
	s.ConfigRebuild = "always"

	s.Types = []string{}
	s.Attributes = make(map[string]string)
	s.Variables = make(map[string]string)
	s.Gateways = make(map[string]int)

	if initFlags.Name != "" {
		s.SanName = initFlags.Name
	}

	if err = writeInstanceConfig(s); err != nil {
		return
	}

	names := []string{s.Name()}
	e := []string{}

	// apply any extra args to settings
	if len(params) > 0 {
		if err = commandSet(San, names, params); err != nil {
			return
		}
		s.Load()
	}

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		if err = createInstanceCert(s); err != nil {
			return
		}
	}

	s.Rebuild(true)

	if initFlags.StartSAN {
		commandInstall(component.ParseComponentName(s.SanType), e, e)
	}

	return nil
}

// rebuild the netprobe.setup.xml file
//
// we do a dance if there is a change in TLS setup and we use default ports
func (s *Sans) Rebuild(initial bool) (err error) {
	if s.ConfigRebuild == "never" {
		return
	}

	if !(s.ConfigRebuild == "always" || (initial && s.ConfigRebuild == "initial")) {
		return
	}

	// recheck check certs/keys
	var changed bool
	secure := s.SanCert != "" && s.SanKey != ""
	for gw := range s.Gateways {
		port := s.Gateways[gw]
		if secure && port == 7039 {
			port = 7038
			changed = true
		} else if !secure && port == 7038 {
			port = 7039
			changed = true
		}
		s.Gateways[gw] = port
	}
	if changed {
		if err := writeInstanceConfig(s); err != nil {
			return err
		}
	}
	return createConfigFromTemplate(s, filepath.Join(s.Home(), "netprobe.setup.xml"), SanDefaultTemplate, SanTemplate)
}

func (s *Sans) Command() (args, env []string) {
	logFile := instance.LogFile(s)
	args = []string{
		s.Name(),
		"-listenip", "none",
		"-port", strconv.Itoa(s.SanPort),
		"-setup", "netprobe.setup.xml",
		"-setup-interval", "300",
	}

	// add environment variables to use in setup file substitution
	env = append(env, "LOG_FILENAME="+logFile)

	if s.SanCert != "" {
		args = append(args, "-secure", "-ssl-certificate", s.SanCert)
	}

	if s.SanKey != "" {
		args = append(args, "-ssl-certificate-key", s.SanKey)
	}

	return
}

func (s *Sans) Reload(params []string) (err error) {
	return ErrNotSupported
}
