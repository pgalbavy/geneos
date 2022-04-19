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
	"strconv"
	"sync"

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
	DefaultSettings: map[string]string{
		"FAPortRange": "7030,7100-",
		"FACleanList": "*.old",
		"FAPurgeList": "fileagent.log:fileagent.txt",
	},
}

type FileAgents struct {
	instance.Instance
	BinSuffix string `default:"agent.linux_64"`
	FAHome    string `default:"{{join .RemoteRoot \"fileagent\" \"fileagents\" .InstanceName}}"`
	FABins    string `default:"{{join .RemoteRoot \"packages\" \"fileagent\"}}"`
	FABase    string `default:"active_prod"`
	FAExec    string `default:"{{join .FABins .FABase .BinSuffix}}"`
	FALogD    string `json:",omitempty"`
	FALogF    string `default:"fileagent.log"`
	FAPort    int    `default:"7030"`
	FAMode    string `json:",omitempty"`
	FAOpts    string `json:",omitempty"`
	FALibs    string `default:"{{join .FABins .FABase \"lib64\"}}:{{join .FABins .FABase}}"`
	FAUser    string `json:",omitempty"`
	FACert    string `json:",omitempty"`
	FAKey     string `json:",omitempty"`
}

func init() {
	geneos.RegisterComponent(&FileAgent, New)
	FileAgent.RegisterDirs([]string{
		"packages/fileagent",
		"fileagent/fileagents",
	})
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
	c.InstanceRemote = r
	c.RemoteRoot = r.GeneosRoot()
	c.Component = &FileAgent
	c.InstanceName = local
	if err := instance.SetDefaults(c); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	c.InstanceHost = host.Name(r.String())
	fileagents.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (n *FileAgents) Type() *geneos.Component {
	return n.Component
}

func (n *FileAgents) Name() string {
	return n.InstanceName
}

func (n *FileAgents) Location() host.Name {
	return n.InstanceHost
}

func (n *FileAgents) Home() string {
	return n.FAHome
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n *FileAgents) Prefix(field string) string {
	return "FA" + field
}

func (n *FileAgents) Remote() *host.Host {
	return n.InstanceRemote
}

func (n *FileAgents) Base() *instance.Instance {
	return &n.Instance
}

func (n *FileAgents) String() string {
	return n.Type().String() + ":" + n.InstanceName + "@" + n.Location().String()
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
	fileagents.Delete(n.Name() + "@" + n.Location().String())
	n.ConfigLoaded = false
	return
}

func (n *FileAgents) Loaded() bool {
	return n.ConfigLoaded
}

func (n *FileAgents) Add(username string, params []string, tmpl string) (err error) {
	n.FAPort = instance.NextPort(n.Remote(), &FileAgent)
	n.FAUser = username

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
		"-port", strconv.Itoa(c.FAPort),
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
