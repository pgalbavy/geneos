package main

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
)

// definitions and access methods for the generic component types

type Component string

const (
	None    Component = ""
	Unknown Component = "unknown"
)

type Components struct {
	// function to call from 'init' and 'add remote' commands to set-up environment
	// arg is the name of the remote
	Initialise func(RemoteName)

	// function to create a new instance of component
	New func(string) Instances

	ComponentType    Component
	ParentType       Component
	ComponentMatches []string
	RealComponent    bool
	DownloadBase     string
}

func init() {
	RegisterComponent(Components{
		ComponentType:    None,
		ParentType:       None,
		ComponentMatches: []string{"", "all", "any"},
		RealComponent:    false,
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
	Location() RemoteName
	Prefix(string) string

	Add(string, []string, string) error
	Command() ([]string, []string)
	Clean(bool, []string) error
	Reload(params []string) (err error)
	Rebuild() error
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
	InstanceLocation RemoteName `default:"local" json:"-"`
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
func RealComponents() (cts []Component) {
	for ct, c := range components {
		if c.RealComponent {
			cts = append(cts, ct)
		}
	}
	return
}

func (ct Component) String() (name string) {
	return string(ct)
}

// return the component type by iterating over all the
// names registered by components. case sensitive.
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

// register directories that need to be created in the
// root of the install (by init)
func RegisterDirs(dirs []string) {
	initDirs = append(initDirs, dirs...)
}

// register setting with their defaults
func RegisterSettings(settings GlobalSettings) {
	for k, v := range settings {
		GlobalConfig[k] = v
	}
}

// Return a slice of all instanceNames for a given Component. No
// checking is done to validate that the directory is a populated
// instance.
func (ct Component) instanceNames(remote RemoteName) (components []string) {
	var files []fs.DirEntry

	if remote == ALL {
		for _, r := range allRemotes() {
			components = append(components, ct.instanceNames(r)...)
		}
		return
	}

	if ct == None {
		for _, t := range RealComponents() {
			d, _ := readDir(remote, t.componentDir(remote))
			files = append(files, d...)
		}
	} else {
		files, _ = readDir(remote, ct.componentDir(remote))
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})
	for _, file := range files {
		if file.IsDir() {
			if ct == Remote {
				components = append(components, file.Name())
			} else {
				components = append(components, file.Name()+"@"+remote.String())
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

// given a component type and a slice of args, call the function for each arg
//
// rely on NewComponent() checking the component type and returning a slice
// of all matching components for a single name in an arg (e.g all instances
// called 'thisserver')
//
// try to use go routines here - mutexes required
func (ct Component) loopCommand(fn func(Instances, []string) error, args []string, params []string) (err error) {
	for _, name := range args {
		for _, c := range ct.instanceMatches(name) {
			if err = fn(c, params); err != nil && !errors.Is(err, ErrProcNotExist) {
				log.Println(c.Type(), c.Name(), err)
			}
		}
	}
	return nil
}

// construct and return a slice of a/all component types that have
// a matching name
// if ct == None, check all real types
//
// change - loadconfig too
func (ct Component) instanceMatches(name string) (c []Instances) {
	var cs []Component
	local, remote := splitInstanceName(name)

	if ct == None {
		for _, t := range RealComponents() {
			c = append(c, t.instanceMatches(name)...)
		}
		return
	}

	for _, dir := range ct.instanceNames(remote) {
		// for case insensitive match change to EqualFold here
		ldir, _ := splitInstanceName(dir)
		if filepath.Base(ldir) == local {
			cs = append(cs, ct)
		}
	}

	for _, cm := range cs {
		i, err := cm.getInstance(name)
		if err != nil {
			log.Fatalln(err)
		}
		c = append(c, i)
	}

	return
}

func (ct Component) getInstance(name string) (c Instances, err error) {
	c = ct.New(name)
	err = loadConfig(c)
	return
}

// Return the base directory for a Component
// ct cannot be None
func (ct Component) componentDir(remote RemoteName) string {
	if ct == None {
		logError.Fatalln(ct, ErrNotSupported)
	}
	switch ct {
	case Remote:
		return GeneosPath(LOCAL, ct.String()+"s")
	default:
		return GeneosPath(remote, ct.String(), ct.String()+"s")
	}
}

// return a slice of initialised instances for a given component type
func (ct Component) instances(remote RemoteName) (confs []Instances) {
	switch ct {
	case None:
		for _, ct := range RealComponents() {
			confs = append(confs, ct.instances(remote)...)
		}
	default:
		for _, name := range ct.instanceNames(remote) {
			confs = append(confs, ct.New(name))
		}
	}

	return
}

func (ct Component) exists(name string) bool {
	if name == LOCAL.String() {
		return true
	}

	_, remote := splitInstanceName(name)
	// first, does remote exist?

	if remote != LOCAL {
		_, err := Remote.getInstance(remote.String())
		if err != nil {
			return false
		}
	}

	_, err := ct.getInstance(name)
	return err == nil
}
