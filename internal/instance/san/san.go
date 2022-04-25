package san

import (
	_ "embed"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/internal/instance/fa2"
	"wonderland.org/geneos/internal/instance/netprobe"
	"wonderland.org/geneos/pkg/logger"
)

var San geneos.Component = geneos.Component{
	Initialise:       Init,
	Name:             "san",
	RelatedTypes:     []*geneos.Component{&netprobe.Netprobe, &fa2.FA2},
	ComponentMatches: []string{"san", "sans"},
	RealComponent:    true,
	DownloadBase:     "Netprobe",
	PortRange:        "SanPortRange",
	CleanList:        "SanCleanList",
	PurgeList:        "SanPurgeList",
	Defaults: []string{
		"binsuffix={{if eq .santype \"fa2\"}}fix-analyser2-{{end}}netprobe.linux_64",
		"sanhome={{join .root \"san\" \"sans\" .name}}",
		"sanbins={{join .root \"packages\" .santype}}",
		"sanbase=active_prod",
		"sanexec={{join .sanbins .sanbase .binsuffix}}",
		"sanlogf=san.log",
		"sanport=7036",
		"sanlibs={{join .sanbins .sanbase \"lib64\"}}:{{join .sanbins .sanbase}}",
		"sanname={{.name}}",
	},
	GlobalSettings: map[string]string{
		"SanPortRange": "7036,7100-",
		"SanCleanList": "*.old",
		"SanPurgeList": "san.log:san.txt:*.snooze:*.user_assignment",
	},
	Directories: []string{
		"packages/netprobe",
		"san/sans",
		"san/templates",
	},
}

type Sans instance.Instance

//go:embed templates/netprobe.setup.xml.gotmpl
var SanTemplate []byte

const SanDefaultTemplate = "netprobe.setup.xml.gotmpl"

func init() {
	geneos.RegisterComponent(&San, New)
}

func Init(r *host.Host, ct *geneos.Component) {
	// copy default template to directory
	if err := geneos.MakeComponentDirs(r, ct); err != nil {
		logger.Error.Fatalln(err)
	}
	if err := r.WriteFile(r.GeneosPath(ct.String(), "templates", SanDefaultTemplate), SanTemplate, 0664); err != nil {
		logger.Error.Fatalln(err)
	}
}

var sans sync.Map

func New(name string) geneos.Instance {
	ct, local, r := instance.SplitName(name, host.LOCAL)
	s, ok := sans.Load(r.FullName(local))
	if ok {
		sn, ok := s.(*Sans)
		if ok {
			return sn
		}
	}
	c := &Sans{}
	c.Conf = viper.New()
	c.InstanceHost = r
	// c.root = r.V().GetString("geneos")
	c.Component = &San
	c.V().SetDefault("santype", "netprobe")
	if ct != nil {
		c.V().SetDefault("santype", ct.Name)
	}
	if err := instance.SetDefaults(c, local); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	sans.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (s *Sans) Type() *geneos.Component {
	return s.Component
}

func (s *Sans) Name() string {
	return s.V().GetString("name")
}

func (s *Sans) Home() string {
	return s.V().GetString("sanhome")
}

// Prefix() takes the string argument and adds any component type specific prefix
func (s *Sans) Prefix(field string) string {
	return strings.ToLower("san" + field)
}

func (s *Sans) Host() *host.Host {
	return s.InstanceHost
}

func (s *Sans) String() string {
	return s.Type().String() + ":" + s.Name() + "@" + s.Host().String()
}

func (s *Sans) Load() (err error) {
	if s.ConfigLoaded {
		return
	}
	err = instance.LoadConfig(s)
	s.ConfigLoaded = err == nil
	return
}

func (s *Sans) Unload() (err error) {
	sans.Delete(s.Name() + "@" + s.Host().String())
	s.ConfigLoaded = false
	return
}

func (s *Sans) Loaded() bool {
	return s.ConfigLoaded
}

func (s *Sans) V() *viper.Viper {
	return s.Conf
}

func (s *Sans) Add(username string, params []string, tmpl string) (err error) {
	s.V().Set("sanport", instance.NextPort(s.InstanceHost, &San))
	s.V().Set("sanuser", username)
	s.V().Set("configrebuild", "always")

	s.V().Set("types", []string{})
	s.V().Set("attributes", make(map[string]string))
	s.V().Set("variables", make(map[string]string))
	s.V().Set("gateways", make(map[string]int))

	// if initFlags.Name != "" {
	// 	s.SanName = initFlags.Name
	// }

	if err = instance.WriteConfig(s); err != nil {
		return
	}

	// apply any extra args to settings
	// names := []string{s.Name()}
	// if len(params) > 0 {
	// 	if err = commandSet(San, names, params); err != nil {
	// 		return
	// 	}
	// 	s.Load()
	// }

	// check tls config, create certs if found
	if _, err = instance.ReadSigningCert(); err == nil {
		if err = instance.CreateCert(s); err != nil {
			return
		}
	}

	s.Rebuild(true)

	// e := []string{}
	// if initFlags.StartSAN {
	// 	commandInstall(s.SanType, e, e)
	// }

	return nil
}

// rebuild the netprobe.setup.xml file
//
// we do a dance if there is a change in TLS setup and we use default ports
func (s *Sans) Rebuild(initial bool) (err error) {
	configrebuild := s.V().GetString("configrebuild")
	if configrebuild == "never" {
		return
	}

	if !(configrebuild == "always" || (initial && configrebuild == "initial")) {
		return
	}

	// recheck check certs/keys
	var changed bool
	secure := s.V().GetString("sancert") != "" && s.V().GetString("sankey") != ""
	gws, ok := s.V().Get("gateways").(map[string]int)
	if !ok {
		return geneos.ErrInvalidArgs
	}
	for gw := range gws {
		port := gws[gw]
		if secure && port == 7039 {
			port = 7038
			changed = true
		} else if !secure && port == 7038 {
			port = 7039
			changed = true
		}
		gws[gw] = port
	}
	if changed {
		s.V().Set("gateways", gws)
		if err := instance.WriteConfig(s); err != nil {
			return err
		}
	}
	return instance.CreateConfigFromTemplate(s, filepath.Join(s.Home(), "netprobe.setup.xml"), SanDefaultTemplate, SanTemplate)
}

func (s *Sans) Command() (args, env []string) {
	logFile := instance.LogFile(s)
	args = []string{
		s.Name(),
		"-listenip", "none",
		"-port", s.V().GetString("sanport"),
		"-setup", "netprobe.setup.xml",
		"-setup-interval", "300",
	}

	// add environment variables to use in setup file substitution
	env = append(env, "LOG_FILENAME="+logFile)

	if s.V().GetString("sancert") != "" {
		args = append(args, "-secure", "-ssl-certificate", s.V().GetString("sancert"))
	}

	if s.V().GetString("sankey") != "" {
		args = append(args, "-ssl-certificate-key", s.V().GetString("sankey"))
	}

	return
}

func (s *Sans) Reload(params []string) (err error) {
	return geneos.ErrNotSupported
}