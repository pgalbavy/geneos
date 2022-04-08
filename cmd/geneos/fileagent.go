package main

// Use this file as a template to add a new component.
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
)

const FileAgent Component = "fileagent"

type FileAgents struct {
	InstanceBase
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
	RegisterComponent(Components{
		New:              NewFileAgent,
		ComponentType:    FileAgent,
		RelatedTypes:     nil,
		ComponentMatches: []string{"fileagent", "fileagents", "file-agent"},
		RealComponent:    true,
		DownloadBase:     "Fix+Analyser+File+Agent",
		PortRange:        "FAPortRange",
		CleanList:        "FACleanList",
		PurgeList:        "FAPurgeList",
	})
	FileAgent.RegisterDirs([]string{
		"packages/fileagent",
		"fileagent/fileagents",
	})
	RegisterDefaultSettings(GlobalSettings{
		"FAPortRange": "7030,7100-",
		"FACleanList": "*.old",
		"FAPurgeList": "fileagent.log:fileagent.txt",
	})
}

func NewFileAgent(name string) Instances {
	_, local, r := SplitInstanceName(name, rLOCAL)
	c := &FileAgents{}
	c.InstanceRemote = r
	c.RemoteRoot = r.GeneosRoot()
	c.InstanceType = FileAgent.String()
	c.InstanceName = local
	if err := setDefaults(&c); err != nil {
		logError.Fatalln(c, "setDefauls():", err)
	}
	c.InstanceLocation = RemoteName(r.InstanceName)
	return c
}

// interface method set

// Return the Component for an Instance
func (n *FileAgents) Type() Component {
	return parseComponentName(n.InstanceType)
}

func (n *FileAgents) Name() string {
	return n.InstanceName
}

func (n *FileAgents) Location() RemoteName {
	return n.InstanceLocation
}

func (n *FileAgents) Home() string {
	return n.FAHome
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n *FileAgents) Prefix(field string) string {
	return "FA" + field
}

func (n *FileAgents) Remote() *Remotes {
	return n.InstanceRemote
}

func (n *FileAgents) Base() *InstanceBase {
	return &n.InstanceBase
}

func (n *FileAgents) String() string {
	return n.Type().String() + ":" + n.InstanceName + "@" + n.Location().String()
}

func (n *FileAgents) Load() (err error) {
	if n.ConfigLoaded {
		return
	}
	err = loadConfig(n)
	n.ConfigLoaded = err == nil
	return
}

func (n *FileAgents) Unload() (err error) {
	n.ConfigLoaded = false
	return
}

func (n *FileAgents) Loaded() bool {
	return n.ConfigLoaded
}

func (n *FileAgents) Add(username string, params []string, tmpl string) (err error) {
	n.FAPort = n.InstanceRemote.nextPort(FileAgent)
	n.FAUser = username

	if err = writeInstanceConfig(n); err != nil {
		logError.Fatalln(err)
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

func (c *FileAgents) Command() (args, env []string) {
	logFile := getLogfilePath(c)
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
	return ErrNotSupported
}

func (c *FileAgents) Rebuild(initial bool) error {
	return ErrNotSupported
}
