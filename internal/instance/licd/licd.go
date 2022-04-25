package licd

import (
	"strings"
	"sync"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/pkg/logger"
)

var Licd geneos.Component = geneos.Component{
	Name:             "licd",
	RelatedTypes:     nil,
	ComponentMatches: []string{"licd", "licds"},
	RealComponent:    true,
	DownloadBase:     "Licence+Daemon",
	PortRange:        "LicdPortRange",
	CleanList:        "LicdCleanList",
	PurgeList:        "LicdPurgeList",
	Defaults: []string{
		"binsuffix=licd.linux_64",
		"licdhome={{join .root \"licd\" \"licds\" .name}}",
		"licdbins={{join .root \"packages\" \"licd\"}}",
		"licdbase=active_prod",
		"licdexec={{join .licdbins .licdbase .binsuffix}}",
		"licdlogf=licd.log",
		"licdport=7041",
		"licdlibs={{join .licdbins .licdbase \"lib64\"}}",
	},
	GlobalSettings: map[string]string{
		"LicdPortRange": "7041,7100-",
		"LicdCleanList": "*.old",
		"LicdPurgeList": "licd.log:licd.txt",
	},
	Directories: []string{
		"packages/licd",
		"licd/licds",
	},
}

type Licds instance.Instance

func init() {
	geneos.RegisterComponent(&Licd, New)
}

var licds sync.Map

func New(name string) geneos.Instance {
	_, local, r := instance.SplitName(name, host.LOCAL)
	l, ok := licds.Load(r.FullName(local))
	if ok {
		lc, ok := l.(*Licds)
		if ok {
			return lc
		}
	}
	c := &Licds{}
	c.Conf = viper.New()
	c.InstanceHost = r
	// c.root = r.V().GetString("geneos")
	c.Component = &Licd
	if err := instance.SetDefaults(c, local); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	licds.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (l *Licds) Type() *geneos.Component {
	return l.Component
}

func (l *Licds) Name() string {
	return l.V().GetString("name")
}

func (l *Licds) Home() string {
	return l.V().GetString("licdhome")
}

func (l *Licds) Prefix(field string) string {
	return strings.ToLower("Licd" + field)
}

func (l *Licds) Host() *host.Host {
	return l.InstanceHost
}

func (l *Licds) String() string {
	return l.Type().String() + ":" + l.Name() + "@" + l.Host().String()
}

func (l *Licds) Load() (err error) {
	if l.ConfigLoaded {
		return
	}
	err = instance.LoadConfig(l)
	l.ConfigLoaded = err == nil
	return
}

func (l *Licds) Unload() (err error) {
	licds.Delete(l.Name() + "@" + l.Host().String())
	l.ConfigLoaded = false
	return
}

func (l *Licds) Loaded() bool {
	return l.ConfigLoaded
}

func (l *Licds) V() *viper.Viper {
	return l.Conf
}

func (l *Licds) Add(username string, params []string, tmpl string) (err error) {
	l.V().Set("licdport", instance.NextPort(l.InstanceHost, &Licd))
	l.V().Set("licduser", username)

	if err = instance.WriteConfig(l); err != nil {
		logger.Error.Fatalln(err)
	}

	// apply any extra args to settings
	// if len(params) > 0 {
	// 	if err = commandSet(Licd, []string{l.Name()}, params); err != nil {
	// 		return
	// 	}
	// 	l.Load()
	// }

	// check tls config, create certs if found
	if _, err = instance.ReadSigningCert(); err == nil {
		if err = instance.CreateCert(l); err != nil {
			return
		}
	}

	// default config XML etc.
	return nil
}

func (l *Licds) Command() (args, env []string) {
	args = []string{
		l.Name(),
		"-port", l.V().GetString("licdport"),
		"-log", instance.LogFile(l),
	}

	if l.V().GetString("licdcert") != "" {
		args = append(args, "-secure", "-ssl-certificate", l.V().GetString("licdcert"))
	}

	if l.V().GetString("licdkey") != "" {
		args = append(args, "-ssl-certificate-key", l.V().GetString("licdkey"))
	}

	return
}

func (l *Licds) Reload(params []string) (err error) {
	return geneos.ErrNotSupported
}

func (l *Licds) Rebuild(initial bool) error {
	return geneos.ErrNotSupported
}
