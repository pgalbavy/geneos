package main

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"sync"
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
	Initialise func(*Remotes)

	// function to create a new instance of component
	New func(string) Instances

	ComponentType    Component
	RelatedTypes     []Component
	ComponentMatches []string
	RealComponent    bool
	DownloadBase     string
	PortRange        Global
	CleanList        Global
	PurgeList        Global
}

func init() {
	RegisterComponent(Components{
		ComponentType:    None,
		RelatedTypes:     nil,
		ComponentMatches: []string{"", "all", "any"},
		RealComponent:    false,
		DownloadBase:     "",
	})
	RegisterDefaultSettings(GlobalSettings{
		// Root directory for all operations
		"Geneos": "",

		// Root URL for all downloads of software archives
		"DownloadURL": "https://resources.itrsgroup.com/download/latest/",

		// Username to start components if not explicitly defined
		// and we are running with elevated privileges
		//
		// When running as a normal user this is unused and
		// we simply test a defined user against the running user
		//
		// default is owner of Geneos
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
	Remote() *Remotes
	Base() *InstanceBase
	Prefix(string) string
	String() string

	Load() error
	Unload() error
	Loaded() bool

	Add(string, []string, string) error
	Command() ([]string, []string)
	// Clean(bool, []string) error
	Reload(params []string) (err error)
	Rebuild(bool) error
}

// The Common type is the common data shared by all component types
type InstanceBase struct {
	Instances `json:"-"`
	// A mutex, for ongoing changes
	L *sync.RWMutex
	// The Name of an instance. This may be different to the instance
	// directory InstanceName during certain operations, e.g. rename
	InstanceName string `json:"Name"`
	// The remote location name (this is a remote component and not
	// a server name). This is NOT written to the config file as it
	// may change if the remote name changes
	InstanceLocation RemoteName `default:"local" json:"-"`
	InstanceRemote   *Remotes   `json:"-"`
	// The Component Type of an instance
	InstanceType string `json:"-"`
	// The RemoteRoot directory of the Geneos installation. Used in template
	// default settings for component types
	RemoteRoot string `json:"-"`

	// Rebuild options; never / always / initial
	// defaults are differemt for gateway and san but go with a safe option
	ConfigRebuild string `default:"never"`

	// set to true when config successfully loaded
	ConfigLoaded bool `json:"-"`

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

// register a component type
func RegisterComponent(c Components) {
	components[c.ComponentType] = c
}

// register directories that need to be created in the
// root of the install (by init)
func (ct Component) RegisterDirs(dirs []string) {
	initDirs[ct] = dirs
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

// create any missing component registered directories
func (ct Component) makeComponentDirs(r *Remotes) (err error) {
	if r == rALL {
		logError.Fatalln(ErrInvalidArgs)
	}
	for _, d := range initDirs[ct] {
		dir := filepath.Join(r.Geneos, d)
		if err = r.MkdirAll(dir, 0775); err != nil {
			return
		}
	}
	return
}

// register setting with their defaults
func RegisterDefaultSettings(settings GlobalSettings) {
	for k, v := range settings {
		GlobalConfig[k] = v
	}
}

// return a slice instances for a given component type
func (ct Component) GetInstancesForComponent(r *Remotes) (confs []Instances) {
	if ct == None {
		for _, c := range RealComponents() {
			confs = append(confs, c.GetInstancesForComponent(r)...)
		}
		return
	}
	for _, name := range ct.FindNames(r) {
		i, err := ct.GetInstance(name)
		if err != nil {
			continue
		}
		confs = append(confs, i)
	}

	return
}

// given a component type and a slice of args, call the function for each arg
//
// rely on New() checking the component type and returning a slice
// of all matching components for a single name in an arg (e.g all instances
// called 'thisserver')
//
// try to use go routines here - mutexes required
func (ct Component) loopCommand(fn func(Instances, []string) error, args []string, params []string) (err error) {
	n := 0
	logDebug.Println("args, params", args, params)
	for _, name := range args {
		cs := ct.FindInstances(name)
		if len(cs) == 0 {
			log.Println("no matches for", ct, name)
			continue
			// return ErrNotFound
		}
		n++
		for _, c := range cs {
			if err = fn(c, params); err != nil && !errors.Is(err, ErrProcNotFound) && !errors.Is(err, ErrNotSupported) {
				log.Println(c, err)
			}
		}
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Return a slice of all instanceNames for a given Component. No
// checking is done to validate that the directory is a populated
// instance.
//
func (ct Component) FindNames(r *Remotes) (names []string) {
	var files []fs.DirEntry

	if r == rALL {
		for _, r := range AllRemotes() {
			names = append(names, ct.FindNames(r)...)
		}
		logDebug.Println("names:", names)
		return
	}

	if ct == None {
		for _, t := range RealComponents() {
			// ignore errors, we only care about any files found
			d, _ := r.ReadDir(t.ComponentDir(r))
			files = append(files, d...)
		}
	} else {
		// ignore errors, we only care about any files found
		files, _ = r.ReadDir(ct.ComponentDir(r))
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})
	for i, file := range files {
		// skip for values with the same name as previous
		if i > 0 && i < len(files) && file.Name() == files[i-1].Name() {
			continue
		}
		if file.IsDir() {
			if ct == Remote {
				names = append(names, file.Name())
			} else {
				names = append(names, file.Name()+"@"+r.InstanceName)
			}
		}
	}
	logDebug.Println("names:", names)
	return
}

// return an instance of component ct. loads the config.
func (ct Component) GetInstance(name string) (c Instances, err error) {
	if ct == None {
		return nil, ErrInvalidArgs
	}

	cm, ok := components[ct]
	if !ok || cm.New == nil {
		return nil, ErrNotSupported
	}

	c = cm.New(name)
	if c == nil {
		return nil, ErrInvalidArgs
	}
	// err =
	c.Load()
	return
}

// construct and return a slice of a/all component types that have
// a matching name
func (ct Component) FindInstances(name string) (c []Instances) {
	logDebug.Println(ct, name)
	_, local, r := SplitInstanceName(name, rALL)
	if !r.Loaded() {
		logDebug.Println("remote", r, "not loaded")
		return
	}

	if ct == None {
		for _, t := range RealComponents() {
			c = append(c, t.FindInstances(name)...)
		}
		return
	}

	for _, name := range ct.FindNames(r) {
		// logDebug.Println(ct, name)
		// for case insensitive match change to EqualFold here
		_, ldir, _ := SplitInstanceName(name, rALL)
		// logDebug.Println(ldir, local)
		if filepath.Base(ldir) == local {
			i, err := ct.GetInstance(name)
			if err != nil {
				continue
			}
			c = append(c, i)
		}
	}

	logDebug.Println(c)
	return
}

// Looks for exactly one matching instance across types and remotes
// returns Invalid Args if zero of more than 1 match
func (ct Component) FindInstance(name string) (c Instances, err error) {
	list := ct.FindInstances(name)
	if len(list) == 0 {
		err = ErrNotFound
		return
	}
	if len(list) == 1 {
		c = list[0]
		return
	}
	err = ErrInvalidArgs
	return
}

// Return the base directory for a Component
// ct cannot be None
func (ct Component) ComponentDir(r *Remotes) string {
	if ct == None {
		logError.Fatalln(ct, ErrNotSupported)
	}
	switch ct {
	case Remote:
		return rLOCAL.GeneosPath(ct.String() + "s")
	default:
		return r.GeneosPath(ct.String(), ct.String()+"s")
	}
}

func InstanceFileWithExt(c Instances, extension string) (path string) {
	return filepath.Join(c.Home(), c.Type().String()+"."+extension)
}
