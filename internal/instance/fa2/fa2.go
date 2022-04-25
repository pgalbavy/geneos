package fa2

import (
	"strings"
	"sync"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/pkg/logger"
)

var FA2 geneos.Component = geneos.Component{
	Name:             "fa2",
	RelatedTypes:     nil,
	ComponentMatches: []string{"fa2", "fixanalyser", "fixanalyzer", "fixanalyser2-netprobe"},
	RealComponent:    true,
	DownloadBase:     "Fix+Analyser+2+Netprobe",
	PortRange:        "FA2PortRange",
	CleanList:        "FA2CleanList",
	PurgeList:        "FA2PurgeList",
	Defaults: []string{
		"binsuffix=fix-analyser2-netprobe.linux_64",
		"fa2home={{join .root \"fa2\" \"fa2s\" .name}}",
		"fa2bins={{join .root \"packages\" \"fa2\"}}",
		"fa2base=active_prod",
		"fa2exec={{join .fa2bins .fa2base .binsuffix}}",
		"fa2logf=fa2.log",
		"fa2port=7036",
		"fa2libs={{join .fa2bins .fa2base \"lib64\"}}:{{join .fa2bins .fa2base}}",
	},
	GlobalSettings: map[string]string{
		"FA2PortRange": "7030,7100-",
		"FA2CleanList": "*.old",
		"FA2PurgeList": "fa2.log:fa2.txt:*.snooze:*.user_assignment",
	},
	Directories: []string{
		"packages/fa2",
		"fa2/fa2s",
	},
}

type FA2s instance.Instance

func init() {
	geneos.RegisterComponent(&FA2, New)
}

var fa2s sync.Map

func New(name string) geneos.Instance {
	_, local, r := instance.SplitName(name, host.LOCAL)
	f, ok := fa2s.Load(r.FullName(local))
	if ok {
		fa, ok := f.(*FA2s)
		if ok {
			return fa
		}
	}
	c := &FA2s{}
	c.Conf = viper.New()
	c.InstanceHost = r
	// c.root = r.V().GetString("geneos")
	c.Component = &FA2
	if err := instance.SetDefaults(c, local); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	fa2s.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (n *FA2s) Type() *geneos.Component {
	return n.Component
}

func (n *FA2s) Name() string {
	return n.V().GetString("name")
}

func (n *FA2s) Home() string {
	return n.V().GetString("fa2Home")
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n *FA2s) Prefix(field string) string {
	return strings.ToLower("fa2" + field)
}

func (n *FA2s) Host() *host.Host {
	return n.InstanceHost
}

func (n *FA2s) String() string {
	return n.Type().String() + ":" + n.Name() + "@" + n.Host().String()
}

func (n *FA2s) Load() (err error) {
	if n.ConfigLoaded {
		return
	}
	err = instance.LoadConfig(n)
	n.ConfigLoaded = err == nil
	return
}

func (n *FA2s) Unload() (err error) {
	fa2s.Delete(n.Name() + "@" + n.Host().String())
	n.ConfigLoaded = false
	return
}

func (n *FA2s) Loaded() bool {
	return n.ConfigLoaded
}

func (n *FA2s) V() *viper.Viper {
	return n.Conf
}

func (n *FA2s) Add(username string, params []string, tmpl string) (err error) {
	n.V().Set("fa2port", instance.NextPort(n.InstanceHost, &FA2))
	n.V().Set("fa2user", username)

	if err = instance.WriteConfig(n); err != nil {
		return
	}

	// apply any extra args to settings
	// if len(params) > 0 {
	// 	if err = commandSet(san.San, []string{n.Name()}, params); err != nil {
	// 		return
	// 	}
	// 	n.Load()
	// }

	// check tls config, create certs if found
	if _, err = instance.ReadSigningCert(); err == nil {
		if err = instance.CreateCert(n); err != nil {
			return
		}
	}

	// default config XML etc.
	return nil
}

func (n *FA2s) Command() (args, env []string) {
	logFile := instance.LogFile(n)
	args = []string{
		n.Name(),
		"-port", n.V().GetString("fa2port"),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if n.V().GetString("fa2cert") != "" {
		args = append(args, "-secure", "-ssl-certificate", n.V().GetString("fa2cert"))
	}

	if n.V().GetString("fa2key") != "" {
		args = append(args, "-ssl-certificate-key", n.V().GetString("fa2key"))
	}

	return
}

func (n *FA2s) Reload(params []string) (err error) {
	return geneos.ErrNotSupported
}

func (n *FA2s) Rebuild(initial bool) error {
	return geneos.ErrNotSupported
}
