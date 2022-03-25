package main

import (
	"errors"
	"strconv"
)

const Licd Component = "licd"

type Licds struct {
	InstanceBase
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
	RegisterComponent(Components{
		New:              NewLicd,
		ComponentType:    Licd,
		ParentType:       None,
		ComponentMatches: []string{"licd", "licds"},
		RealComponent:    true,
		DownloadBase:     "Licence+Daemon",
	})
	RegisterDirs([]string{
		"packages/licd",
		"licd/licds",
	})
	RegisterSettings(GlobalSettings{
		"LicdPortRange": "7041,7100-",
		"LicdCleanList": "*.old",
		"LicdPurgeList": "licd.log:licd.txt",
	})
}

func NewLicd(name string) Instances {
	_, local, r := SplitInstanceName(name, rLOCAL)
	c := &Licds{}
	c.InstanceRemote = r
	c.RemoteRoot = r.GeneosRoot()
	c.InstanceType = Licd.String()
	c.InstanceName = local
	if err := setDefaults(&c); err != nil {
		log.Fatalln(c, "setDefauls():", err)
	}
	c.InstanceLocation = RemoteName(r.InstanceName)
	return c
}

// interface method set

// Return the Component for an Instance
func (l *Licds) Type() Component {
	return parseComponentName(l.InstanceType)
}

func (l *Licds) Name() string {
	return l.InstanceName
}

func (l *Licds) Location() RemoteName {
	return l.InstanceLocation
}

func (l *Licds) Home() string {
	return l.LicdHome
}

func (l *Licds) Prefix(field string) string {
	return "Licd" + field
}

func (l *Licds) Remote() *Remotes {
	return l.InstanceRemote
}

func (l *Licds) String() string {
	return l.Type().String() + ":" + l.InstanceName + "@" + l.Location().String()
}

func (l *Licds) Load() (err error) {
	if l.ConfigLoaded {
		return
	}
	err = loadConfig(l)
	l.ConfigLoaded = err == nil
	return
}

func (l *Licds) Unload() (err error) {
	l.ConfigLoaded = false
	return
}

func (l *Licds) Loaded() bool {
	return l.ConfigLoaded
}

func (l *Licds) Add(username string, params []string, tmpl string) (err error) {
	l.LicdPort = l.InstanceRemote.nextPort(GlobalConfig["LicdPortRange"])
	l.LicdUser = username

	if err = writeInstanceConfig(l); err != nil {
		logError.Fatalln(err)
	}

	// apply any extra args to settings
	if len(params) > 0 {
		commandSet(Licd, []string{l.Name()}, params)
		l.Load()
	}

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(l)
	}

	// default config XML etc.
	return nil
}

func (l *Licds) Rebuild(initial bool) error {
	return ErrNotSupported
}

func (l *Licds) Command() (args, env []string) {
	args = []string{
		l.Name(),
		"-port", strconv.Itoa(l.LicdPort),
		"-log", getLogfilePath(l),
	}

	if l.LicdCert != "" {
		args = append(args, "-secure", "-ssl-certificate", l.LicdCert)
	}

	if l.LicdKey != "" {
		args = append(args, "-ssl-certificate-key", l.LicdKey)
	}

	return
}

func (l *Licds) Clean(purge bool, params []string) (err error) {
	if purge {
		var stopped bool = true
		err = stopInstance(l, params)
		if err != nil {
			if errors.Is(err, ErrProcNotExist) {
				stopped = false
			} else {
				return err
			}
		}
		if err = deletePaths(l, GlobalConfig["LicdCleanList"]); err != nil {
			return err
		}
		err = deletePaths(l, GlobalConfig["LicdPurgeList"])
		if stopped {
			err = startInstance(l, params)
		}
		return
	}
	return deletePaths(l, GlobalConfig["LicdCleanList"])
}

func (l *Licds) Reload(params []string) (err error) {
	return ErrNotSupported
}
