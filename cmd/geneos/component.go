package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// definitions and access methods for the generic component types

type ComponentType int

const (
	// None - no component supplied or required
	None ComponentType = iota
	// Unknown - doesn't match component type
	Unknown
	Gateway
	Netprobe
	Licd
	Webserver
	Remote
)

// XXX this should become an interface
// but that involves lots of rebuilding.
type ComponentFuncs struct {
	Instance func(string) interface{}
	Command  func(Instance) ([]string, []string)
	Add      func(string, string, []string) (Instance, error)
	Clean    func(Instance, bool, []string) error
	Reload   func(Instance, []string) error
}

type Components map[ComponentType]ComponentFuncs

var components Components = make(Components)

// The Instance type is a placeholder interface that can be passed to
// functions which then use reflection to get and set concrete data
// depending on the underlying component type
type Instance interface {
	// empty
}

// The Common type is the common data shared by all component types
type Common struct {
	Instance `json:"-"`
	// The Name of an instance. This may be different to the instance
	// directory name during certain operations, e.g. rename
	Name string `json:"Name"`
	// The potential remote name (this is a remote component and not
	// a server name)
	Location string `default:"local" json:"Location"`
	// The ComponentType of an instance
	Type string `json:"-"`
	// The root directory of the Geneos installation. Used in template
	// default settings for component types
	Root string `json:"-"`
	// Env is a slice of environment variables, as "KEY=VALUE", for the instance
	Env []string `json:",omitempty"`
}

// currently supported real component types, for looping
// (go doesn't allow const slices, a function is the workaround)
// not including Remote - this is special
func realComponentTypes() []ComponentType {
	return []ComponentType{Gateway, Netprobe, Licd, Webserver}
}

func (ct ComponentType) String() string {
	switch ct {
	case None:
		return "none"
	case Gateway:
		return "gateway"
	case Netprobe:
		return "netprobe"
	case Licd:
		return "licd"
	case Webserver:
		return "webserver"
	case Remote:
		return "remote"
	}
	return "unknown"
}

func parseComponentName(component string) ComponentType {
	switch strings.ToLower(component) {
	case "", "any":
		return None
	case "gateway", "gateways":
		return Gateway
	case "netprobe", "probe", "netprobes", "probes":
		return Netprobe
	case "licd", "licds":
		return Licd
	case "web-server", "webserver", "webservers", "webdashboard", "dashboards":
		return Webserver
	case "remote", "remotes":
		return Remote
	default:
		return Unknown
	}
}

// Return a slice of all instances for a given ComponentType. No checking is done
// to validate that the directory is a populated instance.
//
// No side-effects
func instanceDirsForComponent(remote string, ct ComponentType) []string {
	return sortedInstancesInDir(remote, componentDir(remote, ct))
}

// Return the base directory for a ComponentType
func componentDir(remote string, ct ComponentType) string {
	switch ct {
	case Remote:
		return filepath.Join(RunningConfig.ITRSHome, ct.String()+"s")
	default:
		return filepath.Join(remoteRoot(remote), ct.String(), ct.String()+"s")
	}
}

// Accessor functions

// Return the ComponentType for an Instance
func Type(c Instance) ComponentType {
	return parseComponentName(getString(c, "Type"))
}

func Name(c Instance) string {
	return getString(c, "Name")
}

func Location(c Instance) string {
	return getString(c, "Location")
}

func Home(c Instance) string {
	return getString(c, Prefix(c)+"Home")
}

func Prefix(c Instance) string {
	switch Type(c) {
	case Remote:
		return ""
	default:
	}
	if len(Type(c).String()) < 4 {
		return "Default"
	}
	return strings.Title(Type(c).String()[0:4])
}

// Given a slice of directory entries, sort in place
func sortDirEntries(files []fs.DirEntry) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})
}

// Return a sorted list of sub-directories
func sortedInstancesInDir(remote string, dir string) []string {
	files, _ := readDir(remote, dir)
	sortDirEntries(files)
	components := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			components = append(components, file.Name()+"@"+remote)
		}
	}
	return components
}

// Return a new Instance with the given name, as a slice, initialised
// with whatever defaults the component type requires. If ComponentType
// is None then as a special case we return a slice of all instances that
// match the given name per component type.
//
// When not called with a component type of None, the instance does not
// have to exist on disk.
func newComponent(ct ComponentType, name string) (c []Instance) {
	if ct == None {
		// for _, cts := realComponentTypes() {
		// }
		cs := findInstances(name)
		for _, cm := range cs {
			c = append(c, newComponent(cm, name)...)
		}
		return
	}
	cm, ok := components[ct]
	if !ok {
		logError.Fatalln(ct, ErrNotSupported)
	}
	if cm.Instance == nil {
		return []Instance{}
	}
	return []Instance{cm.Instance(name)}
}
