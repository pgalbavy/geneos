package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"
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
)

type ComponentFuncs struct {
	Instance func(string) interface{}
	Command  func(Instance) ([]string, []string)
	New      func(string, string) (Instance, error)
	Clean    func(Instance, []string) error
	Purge    func(Instance, []string) error
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
	// The ComponentType of an instance
	Type string `json:"-"`
	// The root directory of the Geneos installation. Used in template
	// default settings for component types
	Root string `json:"-"`
	// Env is a slice of environment variables, as "KEY=VALUE", for the instance
	Env []string `json:",omitempty"`
}

// currently supported types, for looping
// (go doesn't allow const slices, a function is the workaround)
func componentTypes() []ComponentType {
	return []ComponentType{Gateway, Netprobe, Licd}
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
	case "webserver", "webservers", "webdashboard", "dashboards":
		return Webserver
	default:
		return Unknown
	}
}

// Return a slice of all directories for a given ComponentType. No checking is done
// to validate that the directory is a populated instance.
//
// No side-effects
func instanceDirs(ct ComponentType) []string {
	return sortedDirs(componentDir(ct))
}

// Return the base directory for a ComponentType
func componentDir(ct ComponentType) string {
	return filepath.Join(RunningConfig.ITRSHome, ct.String(), ct.String()+"s")
}

// Accessor functions

// Return the ComponentType for an Instance
func Type(c Instance) ComponentType {
	return parseComponentName(getString(c, "Type"))
}

func Name(c Instance) string {
	return getString(c, "Name")
}

func Home(c Instance) string {
	return getString(c, Prefix(c)+"Home")
}

func Prefix(c Instance) string {
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
func sortedDirs(dir string) []string {
	files, _ := os.ReadDir(dir)
	sortDirEntries(files)
	components := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			components = append(components, file.Name())
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
func NewComponent(ct ComponentType, name string) (c []Instance) {
	if ct == None {
		cs := findInstances(name)
		for _, cm := range cs {
			c = append(c, NewComponent(cm, name)...)
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

// a template function to support "{{join .X .Y}}"
var textJoinFuncs = template.FuncMap{"join": filepath.Join}

// setDefaults() is a common function called by component New*()
// functions to iterate over the component specific instance
// struct and set the defaults as defined in the 'defaults'
// struct tags.
func setDefaults(c interface{}) (err error) {
	st := reflect.TypeOf(c)
	sv := reflect.ValueOf(c)
	for st.Kind() == reflect.Ptr || st.Kind() == reflect.Interface {
		st = st.Elem()
		sv = sv.Elem()
	}

	n := sv.NumField()

	for i := 0; i < n; i++ {
		ft := st.Field(i)
		fv := sv.Field(i)

		// only set plain strings
		if !fv.CanSet() {
			continue
		}
		if def, ok := ft.Tag.Lookup("default"); ok {
			// treat all defaults as if they are templates
			val, err := template.New(ft.Name).Funcs(textJoinFuncs).Parse(def)
			if err != nil {
				log.Println("parse error:", def)
				continue
			}
			var b bytes.Buffer
			if err = val.Execute(&b, c); err != nil {
				log.Println("cannot convert:", def)
			}
			if err = setField(c, ft.Name, b.String()); err != nil {
				return err
			}
		}
	}
	return
}
