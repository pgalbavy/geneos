package licd

import (
	"strconv"
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
	DefaultSettings: map[string]string{
		"LicdPortRange": "7041,7100-",
		"LicdCleanList": "*.old",
		"LicdPurgeList": "licd.log:licd.txt",
	},
}

type Licds struct {
	instance.Instance
	BinSuffix string `default:"licd.linux_64"`
	LicdHome  string `default:"{{join .RemoteRoot \"licd\" \"licds\" .InstanceName}}"`
	LicdBins  string `default:"{{join .RemoteRoot \"packages\" \"licd\"}}"`
	LicdBase  string `default:"active_prod"`
	LicdExec  string `default:"{{join .LicdBins .LicdBase .BinSuffix}}"`
	LicdLogD  string `json:",omitempty"`
	LicdLogF  string `default:"licd.log"`
	LicdMode  string `json:",omitempty"`
	LicdPort  int    `default:"7041"`
	LicdOpts  string `json:",omitempty"`
	LicdLibs  string `default:"{{join .LicdBins .LicdBase \"lib64\"}}"`
	LicdUser  string `json:",omitempty"`
	LicdCert  string `json:",omitempty"`
	LicdKey   string `json:",omitempty"`
}

func init() {
	geneos.RegisterComponent(&Licd, New)
	Licd.RegisterDirs([]string{
		"packages/licd",
		"licd/licds",
	})
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
	c.InstanceRemote = r
	c.RemoteRoot = r.GeneosRoot()
	c.Component = &Licd
	c.InstanceName = local
	if err := instance.SetDefaults(c); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	c.InstanceHost = host.Name(r.String())
	licds.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (l *Licds) Type() *geneos.Component {
	return l.Component
}

func (l *Licds) Name() string {
	return l.InstanceName
}

func (l *Licds) Location() host.Name {
	return l.InstanceHost
}

func (l *Licds) Home() string {
	return l.LicdHome
}

func (l *Licds) Prefix(field string) string {
	return "Licd" + field
}

func (l *Licds) Remote() *host.Host {
	return l.InstanceRemote
}

func (l *Licds) Base() *instance.Instance {
	return &l.Instance
}

func (l *Licds) String() string {
	return l.Type().String() + ":" + l.InstanceName + "@" + l.Location().String()
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
	licds.Delete(l.Name() + "@" + l.Location().String())
	l.ConfigLoaded = false
	return
}

func (l *Licds) Loaded() bool {
	return l.ConfigLoaded
}

func (l *Licds) Add(username string, params []string, tmpl string) (err error) {
	l.LicdPort = instance.NextPort(l.InstanceRemote, &Licd)
	l.LicdUser = username

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
		"-port", strconv.Itoa(l.LicdPort),
		"-log", instance.LogFile(l.Instance),
	}

	if l.LicdCert != "" {
		args = append(args, "-secure", "-ssl-certificate", l.LicdCert)
	}

	if l.LicdKey != "" {
		args = append(args, "-ssl-certificate-key", l.LicdKey)
	}

	return
}

func (l *Licds) Reload(params []string) (err error) {
	return geneos.ErrNotSupported
}

func (l *Licds) Rebuild(initial bool) error {
	return geneos.ErrNotSupported
}
