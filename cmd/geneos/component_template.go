//go:build ignore

package main

// Use this file as a template to add a new component.
//
// Replace 'NAME' with the camel-cased name of the component, e.g. Gateway
// Replace 'name' with the display name of the component, e.g. gateway
//
// Plural instances of 'Names' / 'names' should be carried through, e.g. Gateways/gateways
//

import (
	"errors"
	"strconv"
)

const NAME Component = "name"

type NAMEs struct {
	InstanceBase
	BinSuffix string `default:"binary.linux_64"`
	NameHome  string `default:"{{join .InstanceRoot \"name\" \"names\" .InstanceName}}"`
	NameBins  string `default:"{{join .InstanceRoot \"packages\" \"name\"}}"`
	NameBase  string `default:"active_prod"`
	NameExec  string `default:"{{join .NameBins .NameBase .BinSuffix}}"`
	NameLogD  string `default:"{{.NameHome}}"`
	NameLogF  string `default:"name.log"`
	NamePort  int    `default:"7036"`
	NameMode  string `json:",omitempty"`
	NameOpts  string `json:",omitempty"`
	NameLibs  string `default:"{{join .NameBins .NameBase \"lib64\"}}:{{join .NameBins .NameBase}}"`
	NameUser  string `json:",omitempty"`
	NameCert  string `json:",omitempty"`
	NameKey   string `json:",omitempty"`
}

const NAMEPortRange = "7036,7100-"

func init() {
	RegisterComponent(&Components{
		New:              NewName,
		ComponentType:    name,
		ComponentMatches: []string{"words", "to", "match"},
		IncludeInLoops:   true,
	})
}

func NewName(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Names{}
	c.InstanceRoot = remoteRoot(remote)
	c.InstanceType = Name.String()
	c.InstanceName = local
	c.InstanceLocation = remote
	setDefaults(&c)
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
	return getString(n, n.Prefix("Home"))
}

// Prefix() takes the string argument and adds any component type specific prefix
func (n Names) Prefix(field string) string {
	return "Name" + field
}

func (n Names) Create(username string, params []string) (err error) {
	c := &n
	port := strconv.Itoa(nextPort(RunningConfig.NamePortRange))
	if err = setField(c, c.Prefix("Port"), port); err != nil {
		return
	}
	if err = setField(c, c.Prefix("User"), username); err != nil {
		return
	}

	writeInstanceConfig(c)

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(c)
	}

	// default config XML etc.
	return nil
}

func (c Names) Command() (args, env []string) {
	certfile := getString(c, c.Prefix("Cert"))
	keyfile := getString(c, c.Prefix("Key"))
	logFile := getLogfilePath(c)
	args = []string{
		c.Name(),
		"-port",
		getIntAsString(c, c.Prefix("Port")),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if certfile != "" {
		args = append(args, "-secure", "-ssl-certificate", certfile)
	}

	if keyfile != "" {
		args = append(args, "-ssl-certificate-key", keyfile)
	}

	return
}

var defaultNameCleanList = "*.old"
var defaultNamePurgeList = "name.log:name.txt:*.snooze:*.user_assignment"

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
		if err = removePathList(c, RunningConfig.NameCleanList); err != nil {
			return err
		}
		err = removePathList(c, RunningConfig.NamePurgeList)
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return removePathList(c, RunningConfig.NameCleanList)
}

func (c Names) Reload(params []string) (err error) {
	return ErrNotSupported
}
