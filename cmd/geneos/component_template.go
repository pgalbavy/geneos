//go:build ignore

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
	"errors"
	"strconv"
)

const Name Component = "name"

type Names struct {
	InstanceBase
	BinSuffix string `default:"binary.linux_64"`
	NameHome  string `default:"{{join .RemoteRoot \"name\" \"names\" .InstanceName}}"`
	NameBins  string `default:"{{join .RemoteRoot \"packages\" \"netprobe\"}}"`
	NameBase  string `default:"active_prod"`
	NameExec  string `default:"{{join .NameBins .NameBase .BinSuffix}}"`
	NameLogD  string `json:",omitempty"`
	NameLogF  string `default:"name.log"`
	NamePort  int    `default:"7036"`
	NameMode  string `json:",omitempty"`
	NameOpts  string `json:",omitempty"`
	NameLibs  string `default:"{{join .NameBins .NameBase \"lib64\"}}:{{join .NameBins .NameBase}}"`
	NameUser  string `json:",omitempty"`
	NameCert  string `json:",omitempty"`
	NameKey   string `json:",omitempty"`
}

func init() {
	RegisterComponent(&Components{
		New:              NewName,
		ComponentType:    name,
		ParentType:       None,
		ComponentMatches: []string{"words", "to", "match"},
		IncludeInLoops:   true,
		DownloadBase:     "Name+Whatever",
	})
	RegisterDirs([]string{
		"",
	})
	RegisterSettings(GlobalSettings{
		"Key": "Value",
	})
}

func NewName(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Names{}
	c.RemoteRoot = remoteRoot(remote)
	c.InstanceType = Name.String()
	c.InstanceName = local
	setDefaults(&c)
	c.InstanceLocation = remote
	c.InstanceRemote = loadRemoteConfig(remote)
	return c
}

// interface method set

// Return the Component for an Instance
func (n Names) Type() Component {
	return parseComponentName(n.InstanceType)
}

func (n Names) Name() string {
	return n.InstanceName
}

func (n Names) Location() string {
	return n.InstanceLocation
}

func (n Names) Home() string {
	return n.NameHome
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n Names) Prefix(field string) string {
	return "Name" + field
}

func (n Names) Remote() *Remotes {
	return n.InstanceRemote
}

func (n Names) String() string {
	return n.Type().String() + ":" + n.InstanceName + "@" + n.Location().String()
}

func (n Names) Load() (err error) {
	if n.ConfigLoaded {
		return
	}
	err = loadConfig(n)
	n.ConfigLoaded = err == nil
	return
}

func (n Names) Unload() (err error) {
	n.ConfigLoaded = false
	return
}

func (n Names) Loaded() bool {
	return n.ConfigLoaded
}

func (n Names) Add(username string, params []string) (err error) {
	n.NamePort = n.InstanceRemote.nextPort(RunningConfigMap["NamePortRange"])
	n.NameUser = username

	if err = writeInstanceConfig(n); err != nil {
		logError.Fatalln(err)
	}

	// apply any extra args to settings
	if len(params) > 0 {
		commandSet(San, []string{n.Name()}, params)
		loadConfig(&n)
	}

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(n)
	}

	// default config XML etc.
	return nil
}

func (c Names) Command() (args, env []string) {
	logFile := getLogfilePath(c)
	args = []string{
		c.Name(),
		"-port", fmt.Sprintf(c.NamePort),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if c.NameCert != "" {
		args = append(args, "-secure", "-ssl-certificate", c.NameCert)
	}

	if c.NameKey != "" {
		args = append(args, "-ssl-certificate-key", c.NameKey)
	}

	return
}

func (c Names) Clean(purge bool, params []string) (err error) {
	logDebug.Println(c.Type(), c.Name(), "clean")
	if purge {
		var stopped bool = true
		err = stopInstance(c, params)
		if err != nil {
			if errors.Is(err, ErrProcNotExist) {
				stopped = false
			} else {
				return err
			}
		}
		if err = removePathList(c, RunningConfigMap["NameCleanList"]); err != nil {
			return err
		}
		err = removePathList(c, RunningConfigMap["NamePurgeList"])
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return removePathList(c, RunningConfigMap["NameCleanList"])
}

func (c Names) Reload(params []string) (err error) {
	return ErrNotSupported
}

func (c Names) Rebuild(initial bool) error {
	return ErrNotSupported
}
