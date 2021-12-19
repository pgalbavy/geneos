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
	Gateways
	Netprobes
	Licds
	Webservers
)

// currently supported types, for looping
// (go doesn't allow const slices, a function is the workaround)
func componentTypes() []ComponentType {
	return []ComponentType{Gateways, Netprobes, Licds}
}

func (ct ComponentType) String() string {
	switch ct {
	case None:
		return "none"
	case Gateways:
		return "gateway"
	case Netprobes:
		return "netprobe"
	case Licds:
		return "licd"
	case Webservers:
		return "webserver"
	}
	return "unknown"
}

func parseComponentName(component string) ComponentType {
	switch strings.ToLower(component) {
	case "", "any":
		return None
	case "gateway", "gateways":
		return Gateways
	case "netprobe", "probe", "netprobes", "probes":
		return Netprobes
	case "licd", "licds":
		return Licds
	case "webserver", "webservers", "webdashboard", "dashboards":
		return Webservers
	default:
		return Unknown
	}
}

type Instance interface {
	// empty
}

type Components struct {
	Instance `json:"-"`
	Name     string        `json:"Name"`
	Type     ComponentType `json:"-"`
	Root     string        `json:"-"`
	Env      []string      `json:",omitempty"` // environment variables to set
}

// this method does NOT take a Component as it's used to return
// metadata for where to find Components before the underlying
// type is initialised
//
// No side-effects
func instanceDirs(ct ComponentType) []string {
	return dirs(instanceDir(ct))
}

// as above, this method returns metadata before the underlying
// type is initialised
//
// No side-effects
func instanceDir(ct ComponentType) string {
	return filepath.Join(RunningConfig.ITRSHome, ct.String(), ct.String()+"s")
}

func Type(c Instance) ComponentType {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}
	if !fv.IsValid() {
		return None
	}
	v := fv.FieldByName("Type")

	if v.IsValid() {
		return v.Interface().(ComponentType)
	}
	return None
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

func sortDirs(files []fs.DirEntry) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})
}

// return a sorted list of directories
func dirs(dir string) []string {
	files, _ := os.ReadDir(dir)
	sortDirs(files)
	components := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			components = append(components, file.Name())
		}
	}
	return components
}

var textJoinFuncs = template.FuncMap{"join": filepath.Join}

func NewComponent(ct ComponentType, name string) (c []Instance) {
	switch ct {
	case None:
		cs := findInstances(name)
		for _, cm := range cs {
			c = append(c, NewComponent(cm, name)...)
		}
	case Gateways:
		c = []Instance{NewGateway(name)}
	case Netprobes:
		c = []Instance{NewNetprobe(name)}
	case Licds:
		c = []Instance{NewLicd(name)}
	case Webservers:
		log.Println("webserver not supported yet")
	default:
		log.Println("unknown component", ct)
	}
	return
}

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
