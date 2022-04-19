package netprobe

import (
	"strconv"
	"strings"
	"sync"

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
	GlobalSettings: map[string]string{
		"NetprobePortRange": "7036,7100-",
		"NetprobeCleanList": "*.old",
		"NetprobePurgeList": "netprobe.log:netprobe.txt:*.snooze:*.user_assignment",
	},
}

type Netprobes struct {
	instance.Instance
	BinSuffix string `default:"netprobe.linux_64"`
	NetpHome  string `default:"{{join .RemoteRoot \"netprobe\" \"netprobes\" .InstanceName}}"`
	NetpBins  string `default:"{{join .RemoteRoot \"packages\" \"netprobe\"}}"`
	NetpBase  string `default:"active_prod"`
	NetpExec  string `default:"{{join .NetpBins .NetpBase .BinSuffix}}"`
	NetpLogD  string `json:",omitempty"`
	NetpLogF  string `default:"netprobe.log"`
	NetpPort  int    `default:"7036"`
	NetpMode  string `json:",omitempty"`
	NetpOpts  string `json:",omitempty"`
	NetpLibs  string `default:"{{join .NetpBins .NetpBase \"lib64\"}}:{{join .NetpBins .NetpBase}}"`
	NetpUser  string `json:",omitempty"`
	NetpCert  string `json:",omitempty"`
	NetpKey   string `json:",omitempty"`
}

func init() {
	geneos.RegisterComponent(&Netprobe, New)
	Netprobe.RegisterDirs([]string{
		"packages/netprobe",
		"netprobe/netprobes",
	})
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
	c.InstanceRemote = r
	c.RemoteRoot = r.Geneos
	c.Component = &Netprobe
	c.InstanceName = local
	if err := instance.SetDefaults(c); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	c.InstanceHost = host.Name(r.String())
	netprobes.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (n *Netprobes) Type() *geneos.Component {
	return n.Component
}

func (n *Netprobes) Name() string {
	return n.InstanceName
}

func (n *Netprobes) Location() host.Name {
	return n.InstanceHost
}

func (n *Netprobes) Home() string {
	return n.NetpHome
}

func (n *Netprobes) Prefix(field string) string {
	return strings.ToLower("Netp" + field)
}

func (n *Netprobes) Remote() *host.Host {
	return n.InstanceRemote
}

func (n *Netprobes) Base() *instance.Instance {
	return &n.Instance
}

func (n *Netprobes) String() string {
	return n.Type().String() + ":" + n.InstanceName + "@" + n.Location().String()
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
	netprobes.Delete(n.Name() + "@" + n.Location().String())
	n.ConfigLoaded = false
	return
}

func (n *Netprobes) Loaded() bool {
	return n.ConfigLoaded
}

func (n *Netprobes) Add(username string, params []string, tmpl string) (err error) {
	n.NetpPort = instance.NextPort(n.Remote(), &Netprobe)
	n.NetpUser = username

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
		"-port", strconv.Itoa(n.NetpPort),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if n.NetpCert != "" {
		args = append(args, "-secure", "-ssl-certificate", n.NetpCert)
	}

	if n.NetpKey != "" {
		args = append(args, "-ssl-certificate-key", n.NetpKey)
	}

	return
}

func (n *Netprobes) Reload(params []string) (err error) {
	return geneos.ErrNotSupported
}
