package fileagent

// Use this file as a template to add a new geneos.
//
// Replace 'Name' with the camel-cased name of the component, e.g. Gateway
// Replace 'name' with the display name of the component, e.g. gateway
//
// Plural instances of 'Names' / 'names' should be carried through, e.g. Gateways/gateways
//
// Leave InstanceName alone
//

import (
	"strings"
	"sync"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/pkg/logger"
)

var FileAgent geneos.Component = geneos.Component{
	Name:             "fileagent",
	RelatedTypes:     nil,
	ComponentMatches: []string{"fileagent", "fileagents", "file-agent"},
	RealComponent:    true,
	DownloadBase:     "Fix+Analyser+File+Agent",
	PortRange:        "FAPortRange",
	CleanList:        "FACleanList",
	PurgeList:        "FAPurgeList",
	Defaults: []string{
		"binsuffix=agent.linux_64",
		"fahome={{join .root \"fileagent\" \"fileagents\" .name}}",
		"fabins={{join .root \"packages\" \"fileagent\"}}",
		"fabase=active_prod",
		"faexec={{join .fabins .fabase .binsuffix}}",
		"falogf=fileagent.log",
		"faport=7030",
		"falibs={{join .fabins .fabase \"lib64\"}}:{{join .fabins .fabase}}",
	},
	GlobalSettings: map[string]string{
		"FAPortRange": "7030,7100-",
		"FACleanList": "*.old",
		"FAPurgeList": "fileagent.log:fileagent.txt",
	},
	Directories: []string{
		"packages/fileagent",
		"fileagent/fileagents",
	},
}

type FileAgents instance.Instance

func init() {
	geneos.RegisterComponent(&FileAgent, New)
}

var fileagents sync.Map

func New(name string) geneos.Instance {
	_, local, r := instance.SplitName(name, host.LOCAL)
	f, ok := fileagents.Load(r.FullName(local))
	if ok {
		fa, ok := f.(*FileAgents)
		if ok {
			return fa
		}
	}
	c := &FileAgents{}
	c.Conf = viper.New()
	c.InstanceHost = r
	// c.root = r.V().GetString("geneos")
	c.Component = &FileAgent
	if err := instance.SetDefaults(c, local); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	fileagents.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (n *FileAgents) Type() *geneos.Component {
	return n.Component
}

func (n *FileAgents) Name() string {
	return n.V().GetString("name")
}

func (n *FileAgents) Home() string {
	return n.V().GetString("fahome")
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n *FileAgents) Prefix(field string) string {
	return strings.ToLower("fa" + field)
}

func (n *FileAgents) Host() *host.Host {
	return n.InstanceHost
}

func (n *FileAgents) String() string {
	return n.Type().String() + ":" + n.Name() + "@" + n.Host().String()
}

func (n *FileAgents) Load() (err error) {
	if n.ConfigLoaded {
		return
	}
	err = instance.LoadConfig(n)
	n.ConfigLoaded = err == nil
	return
}

func (n *FileAgents) Unload() (err error) {
	fileagents.Delete(n.Name() + "@" + n.Host().String())
	n.ConfigLoaded = false
	return
}

func (n *FileAgents) Loaded() bool {
	return n.ConfigLoaded
}

func (n *FileAgents) V() *viper.Viper {
	return n.Conf
}

func (n *FileAgents) SetConf(v *viper.Viper) {
	n.Conf = v
}

func (n *FileAgents) Add(username string, params []string, tmpl string) (err error) {
	n.V().Set("faport", instance.NextPort(n.Host(), &FileAgent))
	n.V().Set("fauser", username)

	if err = instance.WriteConfig(n); err != nil {
		logger.Error.Fatalln(err)
	}

	// apply any extra args to settings
	// if len(params) > 0 {
	// 	if err = commandSet(San, []string{n.Name()}, params); err != nil {
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

func (c *FileAgents) Command() (args, env []string) {
	logFile := instance.LogFile(c)
	args = []string{
		c.Name(),
		"-port", c.V().GetString("faport"),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	// if c.FACert != "" {
	// 	args = append(args, "-secure", "-ssl-certificate", c.FACert)
	// }

	// if c.FAKey != "" {
	// 	args = append(args, "-ssl-certificate-key", c.FAKey)
	// }

	return
}

func (c *FileAgents) Reload(params []string) (err error) {
	return geneos.ErrNotSupported
}

func (c *FileAgents) Rebuild(initial bool) error {
	return geneos.ErrNotSupported
}
