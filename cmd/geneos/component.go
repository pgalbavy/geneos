package main

import (
	"path/filepath"
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
	RegisterComponent(&Components{
		ComponentType:    None,
		ComponentMatches: []string{"", "all", "any"},
		IncludeInLoops:   false,
		DownloadBase:     "",
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

	Create(string, []string) error
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
	// The potential remote name (this is a remote component and not
	// a server name)
	InstanceLocation string `default:"local" json:"Location"`
	// The Component of an instance
	InstanceType string `json:"-"`
	// The InstanceRoot directory of the Geneos installation. Used in template
	// default settings for component types
	InstanceRoot string `json:"-"`
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
func RegisterComponent(c *Components) {
	components[c.ComponentType] = *c
}

// Return a slice of all instances for a given Component. No checking is done
// to validate that the directory is a populated instance.
//
// No side-effects
func (ct Component) instanceNamesForComponent(remote string) []string {
	return sortedInstancesInDir(remote, ct.componentBaseDir(remote))
}

// Return the base directory for a Component
// ct cannot be None
func (ct Component) componentBaseDir(remote string) string {
	if ct == None {
		logError.Fatalln(ct, ErrNotSupported)
	}
	switch ct {
	case Remote:
		return filepath.Join(RunningConfig.ITRSHome, ct.String()+"s")
	default:
		return filepath.Join(remoteRoot(remote), ct.String(), ct.String()+"s")
	}
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
		for _, dir := range t.instanceNamesForComponent(remote) {
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

// return a slice of all instances, ordered and grouped.
// configurations are not loaded, just the defaults ready for overlay
func allInstances() (confs []Instances) {
	for _, ct := range realComponentTypes() {
		for _, remote := range allRemotes() {
			confs = append(confs, ct.remoteInstances(remote.Name())...)
		}
	}
	return
}
