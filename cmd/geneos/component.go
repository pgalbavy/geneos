package main

import (
	"path/filepath"
	"sort"
)

// definitions and access methods for the generic component types

type Component string

const (
	None    Component = "none"
	Unknown Component = "unknown"
)

type Components struct {
	New func(string) Instances

	ComponentType    Component
	ComponentMatches []string
	IncludeInLoops   bool
	DownloadBase     string
}

func init() {
	RegisterComponent(Components{
		ComponentType:    None,
		ComponentMatches: []string{"", "all", "any"},
		IncludeInLoops:   false,
		DownloadBase:     "",
	})
	RegisterSettings(GlobalSettings{
		// Root directory for all operations
		"ITRSHome": "",

		// Root URL for all downloads of software archives
		"DownloadURL":  "https://resources.itrsgroup.com/download/latest/",
		"DownloadUser": "",
		"DownloadPass": "",

		// Username to start components if not explicitly defined
		// and we are running with elevated privileges
		//
		// When running as a normal user this is unused and
		// we simply test a defined user against the running user
		//
		// default is owner of ITRSHome
		"DefaultUser": "",

		// Path List sperated additions to the reserved names list, over and above
		// any words matched by parseComponentName()
		"ReservedNames": "",

		"PrivateKeys": "id_rsa,id_ecdsa,id_ecdsa_sk,id_ed25519,id_ed25519_sk,id_dsa",
	})
}

type ComponentsMap map[Component]Components

// slice of registered component types for indirect calls
// this should actually become an Interface
var components ComponentsMap = make(ComponentsMap)

// The Instances interface is used by all components through
// the InstancesBase struct below
type Instances interface {
	Name() string
	Home() string
	Type() Component
	Location() string
	Prefix(string) string

	Add(string, []string, string) error
	Command() ([]string, []string)
	Clean(bool, []string) error
	Reload(params []string) (err error)
}

// The Common type is the common data shared by all component types
type InstanceBase struct {
	Instances `json:"-"`
	// The Name of an instance. This may be different to the instance
	// directory InstanceName during certain operations, e.g. rename
	InstanceName string `json:"Name"`
	// The remote location name (this is a remote component and not
	// a server name). This is NOT written to the config file as it
	// may change if the remote name changes
	InstanceLocation string `default:"local" json:"-"`
	// The Component Type of an instance
	InstanceType string `json:"-"`
	// The RemoteRoot directory of the Geneos installation. Used in template
	// default settings for component types
	RemoteRoot string `json:"-"`

	// Env is a slice of environment variables, as "KEY=VALUE", for the instance
	Env []string `json:",omitempty"`
}

// currently supported real component types, for looping
// (go doesn't allow const slices, a function is the workaround)
// not including Remote - this is special
func realComponentTypes() (cts []Component) {
	for ct, c := range components {
		if c.IncludeInLoops {
			cts = append(cts, ct)
		}
	}
	return
}

func (ct Component) String() (name string) {
	return string(ct)
}

func parseComponentName(component string) Component {
	for ct, v := range components {
		for _, m := range v.ComponentMatches {
			if m == component {
				return ct
			}
		}
	}
	return Unknown
}

// register a component type
func RegisterComponent(c Components) {
	components[c.ComponentType] = c
}

func RegisterDirs(dirs []string) {
	initDirs = append(initDirs, dirs...)
}

func RegisterSettings(settings GlobalSettings) {
	for k, v := range settings {
		GlobalConfig[k] = v
	}
}

// Return a slice of all instanceNames for a given Component. No checking is done
// to validate that the directory is a populated instance.
func (ct Component) instanceNames(remote string) (components []string) {
	switch remote {
	case ALL:
		for _, r := range allRemotes() {
			components = append(components, ct.instanceNames(r)...)
		}
	default:
		files, _ := readDir(remote, ct.componentDir(remote))
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name() < files[j].Name()
		})
		for _, file := range files {
			if file.IsDir() {
				if ct == Remote {
					components = append(components, file.Name())
				} else {
					components = append(components, file.Name()+"@"+remote)
				}
			}
		}
	}
	return
}

// return a new instance of component ct
func (ct Component) New(name string) (c Instances) {
	if ct == None {
		logError.Fatalln(ct, ErrNotSupported)
	}

	cm, ok := components[ct]
	if !ok || cm.New == nil {
		logError.Fatalln(ct, ErrNotSupported)
	}
	return cm.New(name)
}

// construct and return a slice of a/all component types that have
// a matching name
// if ct == None, check all real types
func (ct Component) Match(name string) (c []Instances) {
	var cs []Component

	if ct != None {
		return []Instances{ct.New(name)}
	}

	local, remote := splitInstanceName(name)
	for _, t := range realComponentTypes() {
		for _, dir := range t.instanceNames(remote) {
			// for case insensitive match change to EqualFold here
			ldir, _ := splitInstanceName(dir)
			if filepath.Base(ldir) == local {
				cs = append(cs, t)
			}
		}
	}
	for _, cm := range cs {
		c = append(c, cm.New(name))
	}
	return
}

// Return the base directory for a Component
// ct cannot be None
func (ct Component) componentDir(remote string) string {
	if ct == None {
		logError.Fatalln(ct, ErrNotSupported)
	}
	switch ct {
	case Remote:
		return filepath.Join(ITRSHome(), ct.String()+"s")
	default:
		return filepath.Join(remoteRoot(remote), ct.String(), ct.String()+"s")
	}
}

// return a slice of initialised instances for a given component type
func (ct Component) instances(remote string) (confs []Instances) {
	switch ct {
	case None:
		for _, ct := range realComponentTypes() {
			confs = append(confs, ct.instances(remote)...)
		}
	default:
		for _, name := range ct.instanceNames(remote) {
			confs = append(confs, ct.New(name))
		}
	}

	return
}
