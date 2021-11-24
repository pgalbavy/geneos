package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"
)

// definitions and access methods for the generic component types

type Component interface {
	// empty
}

type ComponentType int

const (
	None ComponentType = iota
	Gateway
	Netprobe
	Licd
	Webserver
)

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

func CompType(component string) ComponentType {
	switch strings.ToLower(component) {
	case "gateway":
		return Gateway
	case "netprobe", "probe":
		return Netprobe
	case "licd":
		return Licd
	case "webserver", "webdashboard":
		return Webserver
	default:
		return None
	}
}

type Components struct {
	Component `json:"-"`
	ITRSHome  string        `json:"-"`
	Type      ComponentType `json:"-"`
	Name      string        `json:"-"`
}

// this method does NOT take a Component as it's used to return
// metadata for where to find Components before the underlying
// type is initialised
func RootDirs(comp ComponentType) []string {
	return dirs(RootDir(comp))
}

// as above, this method returns metadata before the underlying
// type is initialised
func RootDir(comp ComponentType) string {
	return filepath.Join(itrsHome, comp.String(), comp.String()+"s")
}

func Type(c Component) ComponentType {
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

func Name(c Component) string {
	return getString(c, "Name")
}

func Home(c Component) string {
	return getStringWithPrefix(c, "Home")
}

func dirs(dir string) []string {
	files, _ := os.ReadDir(dir)
	components := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			components = append(components, file.Name())
		}
	}
	return components
}

func getIntWithPrefix(c Component, name string) string {
	t := Type(c).String()
	prefix := strings.Title(t[0:4])

	v := reflect.ValueOf(c).Elem().FieldByName(prefix + name)
	if v.IsValid() && v.Kind() == reflect.Int {
		return fmt.Sprintf("%v", v.Int())
	}
	return ""
}

func getString(c Component, name string) string {
	v := reflect.ValueOf(c).Elem().FieldByName(name)
	if v.IsValid() && v.Kind() == reflect.String {
		return v.String()
	}
	return ""
}

func getStringWithPrefix(c Component, name string) string {
	t := Type(c).String()
	prefix := strings.Title(t[0:4])

	return getString(c, prefix+name)
}

func getStringsWithPrefix(c Component, names ...string) (fields []string) {
	t := Type(c).String()
	prefix := strings.Title(t[0:4])

	v := reflect.ValueOf(c).Elem()

	fv := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf("abc")), 0, 5)

	for _, name := range names {
		f := v.FieldByName(prefix + name)
		if f.IsValid() {
			switch f.Kind() {
			case reflect.String:
				fv = reflect.Append(fv, f)
				// fields = append(fields, f.String())
			case reflect.Slice:
				fv = reflect.AppendSlice(fv, f)
			}
		}
	}
	fields = fv.Interface().([]string)
	return
}

func setField(c Component, k string, v string) {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}
	fv = fv.FieldByName(k)
	if fv.IsValid() && fv.CanSet() {
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(v)
		case reflect.Int:
			i, _ := strconv.Atoi(v)
			fv.SetInt(int64(i))
		default:
			log.Printf("cannot set %q to a %T\n", k, v)
		}
	} else {
		log.Println("cannot set", k)
	}
}

func setFields(c Component, k string, v []string) {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}
	fv = fv.FieldByName(k)
	if fv.IsValid() && fv.CanSet() {
		reflect.AppendSlice(fv, reflect.ValueOf(v))
		for _, val := range v {
			fv.Set(reflect.Append(fv, reflect.ValueOf(val)))
		}
	}
}

var funcs = template.FuncMap{"join": filepath.Join}

func New(comp ComponentType, name string) (c Component) {
	switch comp {
	case Gateway:
		c = NewGateway(name)
	case Netprobe:
		c = NewNetprobe(name)
	case Licd:
		c = NewLicd(name)
	case Webserver:
		log.Println("webserver not supported yet")
		os.Exit(0)
	default:
		log.Println("unknown component", comp)
		os.Exit(0)
	}
	return
}

func NewComponent(c interface{}) {
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
			if strings.Contains(def, "{{") {
				val, err := template.New(ft.Name).Funcs(funcs).Parse(def)
				if err != nil {
					log.Println("parse error:", def)
					continue
				}

				var b bytes.Buffer
				err = val.Execute(&b, c)
				if err != nil {
					log.Println("cannot convert:", def)
				}
				setField(c, ft.Name, b.String())
			} else {
				setField(c, ft.Name, def)
			}
		}
	}
}
