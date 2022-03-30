package main

import (
	"strconv"
)

const FA2 Component = "fa2"

type FA2s struct {
	InstanceBase
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
	RegisterComponent(Components{
		New:              NewFA2,
		ComponentType:    FA2,
		RelatedTypes:     nil,
		ComponentMatches: []string{"fa2", "fixanalyser", "fixanalyzer", "fixanalyser2-netprobe"},
		RealComponent:    true,
		DownloadBase:     "Fix+Analyser+2+Netprobe",
	})
	FA2.RegisterDirs([]string{
		"packages/fa2",
		"fa2/fa2s",
	})
	RegisterSettings(GlobalSettings{
		"FA2PortRange": "7030,7100-",
		"FA2CleanList": "*.old",
		"FA2PurgeList": "fa2.log:fa2.txt:*.snooze:*.user_assignment",
	})
}

func NewFA2(name string) Instances {
	_, local, r := SplitInstanceName(name, rLOCAL)
	c := &FA2s{}
	c.InstanceRemote = r
	c.RemoteRoot = r.GeneosRoot()
	c.InstanceType = FA2.String()
	c.InstanceName = local
	if err := setDefaults(&c); err != nil {
		log.Fatalln(c, "setDefauls():", err)
	}
	c.InstanceLocation = RemoteName(r.InstanceName)
	return c
}

// interface method set

// Return the Component for an Instance
func (n *FA2s) Type() Component {
	return parseComponentName(n.InstanceType)
}

func (n *FA2s) Name() string {
	return n.InstanceName
}

func (n *FA2s) Location() RemoteName {
	return n.InstanceLocation
}

func (n *FA2s) Home() string {
	return n.FA2Home
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n *FA2s) Prefix(field string) string {
	return "FA2" + field
}

func (n *FA2s) Remote() *Remotes {
	return n.InstanceRemote
}

func (n *FA2s) String() string {
	return n.Type().String() + ":" + n.InstanceName + "@" + n.Location().String()
}

func (n *FA2s) Load() (err error) {
	if n.ConfigLoaded {
		return
	}
	err = loadConfig(n)
	n.ConfigLoaded = err == nil
	return
}

func (n *FA2s) Unload() (err error) {
	n.ConfigLoaded = false
	return
}

func (n *FA2s) Loaded() bool {
	return n.ConfigLoaded
}

func (n *FA2s) Add(username string, params []string, tmpl string) (err error) {
	n.FA2Port = n.InstanceRemote.nextPort(GlobalConfig["FA2PortRange"])
	n.FA2User = username

	if err = writeInstanceConfig(n); err != nil {
		return
	}

	// apply any extra args to settings
	if len(params) > 0 {
		if err = commandSet(San, []string{n.Name()}, params); err != nil {
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

func (n *FA2s) Command() (args, env []string) {
	logFile := getLogfilePath(n)
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

func (n *FA2s) Clean(purge bool, params []string) (err error) {
	var stopped bool

	if !purge {
		if err = deletePaths(n, GlobalConfig["FA2CleanList"]); err == nil {
			log.Println(n, "cleaned")
		}
		return
	}

	if _, err = findInstancePID(n); err == ErrProcNotExist {
		stopped = false
	} else if err = stopInstance(n, params); err != nil {
		return
	} else {
		stopped = true
	}

	if err = deletePaths(n, GlobalConfig["FA2CleanList"]); err != nil {
		return
	}
	if err = deletePaths(n, GlobalConfig["FA2PurgeList"]); err != nil {
		return
	}
	log.Println(n, "fully cleaned")
	if stopped {
		err = startInstance(n, params)
	}
	return
}

func (n *FA2s) Reload(params []string) (err error) {
	return ErrNotSupported
}

func (n *FA2s) Rebuild(initial bool) error {
	return ErrNotSupported
}
