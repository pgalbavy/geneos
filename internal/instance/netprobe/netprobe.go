package netprobe

import (
	"strings"
	"sync"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/pkg/logger"
)

var Netprobe geneos.Component = geneos.Component{
	Name:             "netprobe",
	RelatedTypes:     nil,
	ComponentMatches: []string{"netprobe", "probe", "netprobes", "probes"},
	RealComponent:    true,
	DownloadBase:     "Netprobe",
	PortRange:        "NetprobePortRange",
	CleanList:        "NetprobeCleanList",
	PurgeList:        "NetprobePurgeList",
	Defaults: []string{
		"binsuffix=netprobe.linux_64",
		"netphome={{join .remoteroot \"netprobe\" \"netprobes\" .name}}",
		"netpbins={{join .remoteroot \"packages\" \"netprobe\"}}",
		"netpbase=active_prod",
		"netpexec={{join .netpbins .netpbase .binsuffix}}",
		"netplogf=netprobe.log",
		"netplibs={{join .netpbins .netpbase \"lib64\"}}:{{join .netpbins .netpbase}}",
	},
	GlobalSettings: map[string]string{
		"NetprobePortRange": "7036,7100-",
		"NetprobeCleanList": "*.old",
		"NetprobePurgeList": "netprobe.log:netprobe.txt:*.snooze:*.user_assignment",
	},
	Directories: []string{
		"packages/netprobe",
		"netprobe/netprobes",
	},
}

type Netprobes instance.Instance

func init() {
	geneos.RegisterComponent(&Netprobe, New)
}

var netprobes sync.Map

func New(name string) geneos.Instance {
	_, local, r := instance.SplitName(name, host.LOCAL)
	n, ok := netprobes.Load(r.FullName(local))
	if ok {
		np, ok := n.(*Netprobes)
		if ok {
			return np
		}
	}
	c := &Netprobes{}
	c.Conf = viper.New()
	c.InstanceHost = r
	// c.RemoteRoot = r.V().GetString("geneos")
	c.Component = &Netprobe
	if err := instance.SetDefaults(c, local); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	netprobes.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (n *Netprobes) Type() *geneos.Component {
	return n.Component
}

func (n *Netprobes) Name() string {
	return n.V().GetString("name")
}

func (n *Netprobes) Home() string {
	return n.V().GetString("netphome")
}

func (n *Netprobes) Prefix(field string) string {
	return strings.ToLower("netp" + field)
}

func (n *Netprobes) Host() *host.Host {
	return n.InstanceHost
}

func (n *Netprobes) String() string {
	return n.Type().String() + ":" + n.Name() + "@" + n.Host().String()
}

func (n *Netprobes) Load() (err error) {
	if n.ConfigLoaded {
		return
	}
	err = instance.LoadConfig(n)
	n.ConfigLoaded = err == nil
	return
}

func (n *Netprobes) Unload() (err error) {
	netprobes.Delete(n.Name() + "@" + n.Host().String())
	n.ConfigLoaded = false
	return
}

func (n *Netprobes) Loaded() bool {
	return n.ConfigLoaded
}

func (n *Netprobes) V() *viper.Viper {
	return n.Conf
}

func (n *Netprobes) Add(username string, params []string, tmpl string) (err error) {
	n.V().Set("netpport", instance.NextPort(n.Host(), &Netprobe))
	n.V().Set("netpuser", username)

	if err = instance.WriteConfig(n); err != nil {
		return
	}

	// apply any extra args to settings
	// if len(params) > 0 {
	// 	if err = commandSet(Netprobe, []string{n.Name()}, params); err != nil {
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

func (n *Netprobes) Rebuild(initial bool) error {
	return geneos.ErrNotSupported
}

func (n *Netprobes) Command() (args, env []string) {
	logFile := instance.LogFile(n)
	args = []string{
		n.Name(),
		"-port", n.V().GetString("netpport"),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if n.V().GetString("netpcert") != "" {
		args = append(args, "-secure", "-ssl-certificate", n.V().GetString("netpcert"))
	}

	if n.V().GetString("netpkey") != "" {
		args = append(args, "-ssl-certificate-key", n.V().GetString("netpkey"))
	}

	return
}

func (n *Netprobes) Reload(params []string) (err error) {
	return geneos.ErrNotSupported
}
