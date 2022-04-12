package main

import (
	"strconv"
	"sync"
)

const Netprobe Component = "netprobe"

type Netprobes struct {
	InstanceBase
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
	RegisterComponent(Components{
		New:              NewNetprobe,
		ComponentType:    Netprobe,
		RelatedTypes:     nil,
		ComponentMatches: []string{"netprobe", "probe", "netprobes", "probes"},
		RealComponent:    true,
		DownloadBase:     "Netprobe",
		PortRange:        "NetprobePortRange",
		CleanList:        "NetprobeCleanList",
		PurgeList:        "NetprobePurgeList",
	})
	Netprobe.RegisterDirs([]string{
		"packages/netprobe",
		"netprobe/netprobes",
	})
	RegisterDefaultSettings(GlobalSettings{
		"NetprobePortRange": "7036,7100-",
		"NetprobeCleanList": "*.old",
		"NetprobePurgeList": "netprobe.log:netprobe.txt:*.snooze:*.user_assignment",
	})
}

var netprobes sync.Map

func NewNetprobe(name string) Instance {
	_, local, r := SplitInstanceName(name, rLOCAL)
	n, ok := netprobes.Load(r.FullName(local))
	if ok {
		np, ok := n.(*Netprobes)
		if ok {
			return np
		}
	}
	c := &Netprobes{}
	c.InstanceRemote = r
	c.RemoteRoot = r.GeneosRoot()
	c.InstanceType = Netprobe.String()
	c.InstanceName = local
	if err := setDefaults(&c); err != nil {
		logError.Fatalln(c, "setDefaults():", err)
	}
	c.InstanceLocation = RemoteName(r.InstanceName)
	netprobes.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (n *Netprobes) Type() Component {
	return parseComponentName(n.InstanceType)
}

func (n *Netprobes) Name() string {
	return n.InstanceName
}

func (n *Netprobes) Location() RemoteName {
	return n.InstanceLocation
}

func (n *Netprobes) Home() string {
	return n.NetpHome
}

func (n *Netprobes) Prefix(field string) string {
	return "Netp" + field
}

func (n *Netprobes) Remote() *Remotes {
	return n.InstanceRemote
}

func (n *Netprobes) Base() *InstanceBase {
	return &n.InstanceBase
}

func (n *Netprobes) String() string {
	return n.Type().String() + ":" + n.InstanceName + "@" + n.Location().String()
}

func (n *Netprobes) Load() (err error) {
	if n.ConfigLoaded {
		return
	}
	err = loadConfig(n)
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
	n.NetpPort = n.InstanceRemote.nextPort(Netprobe)
	n.NetpUser = username

	if err = writeInstanceConfig(n); err != nil {
		return
	}

	// apply any extra args to settings
	if len(params) > 0 {
		if err = commandSet(Netprobe, []string{n.Name()}, params); err != nil {
			return
		}
		n.Load()
	}

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		if err = createInstanceCert(n); err != nil {
			return
		}
	}

	// default config XML etc.
	return nil
}

func (n *Netprobes) Rebuild(initial bool) error {
	return ErrNotSupported
}

func (n *Netprobes) Command() (args, env []string) {
	logFile := getLogfilePath(n)
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
	return ErrNotSupported
}
