package fa2

import (
	"strconv"
	"strings"
	"sync"
	"syscall"

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
	GlobalSettings: map[string]string{
		"FA2PortRange": "7030,7100-",
		"FA2CleanList": "*.old",
		"FA2PurgeList": "fa2.log:fa2.txt:*.snooze:*.user_assignment",
	},
}

type FA2s struct {
	instance.Instance
	BinSuffix string `default:"fix-analyser2-netprobe.linux_64"`
	FA2Home   string `default:"{{join .RemoteRoot \"fa2\" \"fa2s\" .InstanceName}}"`
	FA2Bins   string `default:"{{join .RemoteRoot \"packages\" \"fa2\"}}"`
	FA2Base   string `default:"active_prod"`
	FA2Exec   string `default:"{{join .FA2Bins .FA2Base .BinSuffix}}"`
	FA2LogD   string `json:",omitempty"`
	FA2LogF   string `default:"fa2.log"`
	FA2Port   int    `default:"7036"`
	FA2Mode   string `json:",omitempty"`
	FA2Opts   string `json:",omitempty"`
	FA2Libs   string `default:"{{join .FA2Bins .FA2Base \"lib64\"}}:{{join .FA2Bins .FA2Base}}"`
	FA2User   string `json:",omitempty"`
	FA2Cert   string `json:",omitempty"`
	FA2Key    string `json:",omitempty"`
}

func init() {
	geneos.RegisterComponent(&FA2, New)
	FA2.RegisterDirs([]string{
		"packages/fa2",
		"fa2/fa2s",
	})
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
	c.InstanceRemote = r
	c.RemoteRoot = r.Geneos
	c.Component = &FA2
	c.InstanceName = local
	if err := instance.SetDefaults(c); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	c.InstanceHost = host.Name(r.String())
	fa2s.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (n *FA2s) Type() *geneos.Component {
	return n.Component
}

func (n *FA2s) Name() string {
	return n.InstanceName
}

func (n *FA2s) Location() host.Name {
	return n.InstanceHost
}

func (n *FA2s) Home() string {
	return n.FA2Home
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n *FA2s) Prefix(field string) string {
	return strings.ToLower("FA2" + field)
}

func (n *FA2s) Remote() *host.Host {
	return n.InstanceRemote
}

func (n *FA2s) Base() *instance.Instance {
	return &n.Instance
}

func (n *FA2s) String() string {
	return n.Type().String() + ":" + n.InstanceName + "@" + n.Location().String()
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
	fa2s.Delete(n.Name() + "@" + n.Location().String())
	n.ConfigLoaded = false
	return
}

func (n *FA2s) Loaded() bool {
	return n.ConfigLoaded
}

func (n *FA2s) Add(username string, params []string, tmpl string) (err error) {
	n.FA2Port = instance.NextPort(n.InstanceRemote, &FA2)
	n.FA2User = username

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
		"-port", strconv.Itoa(n.FA2Port),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if n.FA2Cert != "" {
		args = append(args, "-secure", "-ssl-certificate", n.FA2Cert)
	}

	if n.FA2Key != "" {
		args = append(args, "-ssl-certificate-key", n.FA2Key)
	}

	return
}

func (n *FA2s) Reload(params []string) (err error) {
	return geneos.ErrNotSupported
}

func (n *FA2s) Rebuild(initial bool) error {
	return geneos.ErrNotSupported
}

func (n *FA2s) Signal(s syscall.Signal) error {
	return instance.Signal(n, s)
}
