package netprobe

import (
	"sync"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/pkg/logger"
)

var Netprobe = geneos.Component{
	Name:             "netprobe",
	RelatedTypes:     nil,
	ComponentMatches: []string{"netprobe", "probe", "netprobes", "probes"},
	RealComponent:    true,
	DownloadBase:     geneos.DownloadBases{Resources: "Netprobe", Nexus: "geneos-netprobe"},
	PortRange:        "NetprobePortRange",
	CleanList:        "NetprobeCleanList",
	PurgeList:        "NetprobePurgeList",
	Aliases: map[string]string{
		"binsuffix": "binary",
		"netphome":  "home",
		"netpbins":  "install",
		"netpbase":  "version",
		"netpexec":  "program",
		"netplogd":  "logdir",
		"netplogf":  "logfile",
		"netpport":  "port",
		"netplibs":  "libpaths",
		"netpcert":  "certificate",
		"netpkey":   "privatekey",
		"netpuser":  "user",
		"netpopts":  "options",
	},
	Defaults: []string{
		"binary=netprobe.linux_64",
		"home={{join .root \"netprobe\" \"netprobes\" .name}}",
		"install={{join .root \"packages\" \"netprobe\"}}",
		"version=active_prod",
		"program={{join .install .version .binary}}",
		"logfile=netprobe.log",
		"libpaths={{join .install .version \"lib64\"}}:{{join .install .version}}",
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
	// c.root = r.V().GetString("geneos")
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
	return n.V().GetString("home")
}

func (n *Netprobes) Prefix() string {
	return "netp"
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

func (n *Netprobes) SetConf(v *viper.Viper) {
	n.Conf = v
}

func (n *Netprobes) Add(username string, tmpl string, port uint16) (err error) {
	if port == 0 {
		port = instance.NextPort(n.Host(), &Netprobe)
	}
	n.V().Set("port", port)
	n.V().Set("user", username)

	if err = instance.WriteConfig(n); err != nil {
		return
	}

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
		"-port", n.V().GetString("port"),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if n.V().GetString("certificate") != "" {
		args = append(args, "-secure", "-ssl-certificate", n.V().GetString("certificate"))
	}

	if n.V().GetString("privatekey") != "" {
		args = append(args, "-ssl-certificate-key", n.V().GetString("privatekey"))
	}

	return
}

func (n *Netprobes) Reload(params []string) (err error) {
	return geneos.ErrNotSupported
}
