package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"
)

// reflect methods to get and set struct fields

func getBool(c interface{}, name string) bool {
	v := reflect.ValueOf(c)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	if !v.IsValid() || v.Kind() != reflect.Struct {
		return false
	}

	v = v.FieldByName(name)
	if v.IsValid() && v.Kind() == reflect.Bool {
		return v.Bool()
	}
	return false
}

func getInt(c interface{}, name string) int64 {
	v := reflect.ValueOf(c)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	if !v.IsValid() || v.Kind() != reflect.Struct {
		return 0
	}

	v = v.FieldByName(name)
	if v.IsValid() && v.Kind() == reflect.Int {
		return v.Int()
	}
	return 0
}

func getString(c interface{}, name string) string {
	v := reflect.ValueOf(c)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	if !v.IsValid() || v.Kind() != reflect.Struct {
		return ""
	}

	v = v.FieldByName(name)
	if v.IsValid() && v.Kind() == reflect.String {
		return v.String()
	}

	return ""
}

func getSliceStrings(c interface{}, name string) (strings []string) {
	v := reflect.ValueOf(c)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil
	}

	v = v.FieldByName(name)
	if v.Type() != reflect.TypeOf(strings) {
		return nil
	}

	return v.Interface().([]string)
}

func setField(c interface{}, k string, v string) (err error) {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}
	if fv.Kind() == reflect.Map {
		return fmt.Errorf("cannot set field in a map")
	}
	fv = fv.FieldByName(k)
	if fv.IsValid() && fv.CanSet() {
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(v)
		case reflect.Int:
			i, _ := strconv.Atoi(v)
			fv.SetInt(int64(i))
		case reflect.Bool:
			if v == "1" || strings.EqualFold(v, "true") {
				fv.SetBool(true)
			} else {
				fv.SetBool(false)
			}
		default:
			return fmt.Errorf("cannot set %q to a %T: %w", k, v, ErrInvalidArgs)
		}
	} else {
		return fmt.Errorf("cannot set %q: %w (isValid=%v, canset=%v)", k, ErrInvalidArgs, fv.IsValid(), fv.CanSet())
	}
	return
}

func setFieldSlice(c interface{}, k string, v []string) (err error) {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}
	fv = fv.FieldByName(k)
	if fv.IsValid() && fv.CanSet() {
		fv.Set(reflect.ValueOf(v))
	}
	return
}

func setStructMap(c interface{}, field string, k string, v string) (err error) {
	fv := reflect.ValueOf(c)
	for fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}

	if fv.Kind() != reflect.Struct {
		return fmt.Errorf("not a struct - cannot set map field")
	}

	fm := fv.FieldByName(field)
	if fm.Kind() != reflect.Map {
		return fmt.Errorf("not a map - cannot set key")
	}

	// initialise and set back into struct
	if fm.IsNil() {
		fm = reflect.MakeMap(reflect.MapOf(fm.Type().Key(), fm.Type().Elem()))
		fv.FieldByName(field).Set(fm)
	}

	var key reflect.Value

	switch fm.Type().Key().Kind() {
	case reflect.Int:
		i, _ := strconv.Atoi(k)
		key = reflect.ValueOf(i)
	case reflect.String:
		key = reflect.ValueOf(k)
	default:
		return fmt.Errorf("cannot use %q as a key: %w", k, ErrInvalidArgs)
	}

	if fm.IsValid() && !fm.IsNil() {
		var val reflect.Value
		if v == "" {
			val = reflect.ValueOf(nil)
		} else {
			switch fm.Type().Elem().Kind() {
			case reflect.String:
				val = reflect.ValueOf(v)
			case reflect.Int:
				i, _ := strconv.Atoi(v)
				val = reflect.ValueOf(i)
			default:
				return fmt.Errorf("cannot set %q to a %T: %w", k, v, ErrInvalidArgs)
			}
		}

		fm.SetMapIndex(key, val)
	} else {
		return fmt.Errorf("cannot set %q: %w (isValid=%v, isNil=%v, canset=%v, type=%v)", k, ErrInvalidArgs, fm.IsValid(), fm.IsNil(), fm.CanSet(), fm.Type())
	}
	return
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

	for _, f := range reflect.VisibleFields(st) {
		fv := sv.FieldByIndex(f.Index)

		if !f.IsExported() {
			continue
		}
		if !fv.CanSet() {
			logDebug.Println("cannot set", f.Name)
			return err
		}
		if def, ok := f.Tag.Lookup("default"); ok {
			// treat all defaults as if they are templates
			val, err := template.New(f.Name).Funcs(textJoinFuncs).Parse(def)
			if err != nil {
				log.Println(c, "setDefaults parse error:", def)
				return err
			}
			var b bytes.Buffer
			if err = val.Execute(&b, c); err != nil {
				log.Println(c, "cannot set defaults:", def)
				return err
			}
			if err = setField(c, f.Name, b.String()); err != nil {
				return err
			}
		}
	}

	return
}

// go through a struct and update directory path prefixes from
// old to new.
// XXX may need to ignore specific fields
func changeDirPrefix(c interface{}, old, new string) (err error) {
	st := reflect.TypeOf(c)
	sv := reflect.ValueOf(c)
	for st.Kind() == reflect.Ptr || st.Kind() == reflect.Interface {
		st = st.Elem()
		sv = sv.Elem()
	}

	for _, f := range reflect.VisibleFields(st) {
		fv := sv.FieldByIndex(f.Index)

		if fv.Kind() != reflect.String {
			continue
		}

		if !fv.CanSet() {
			logDebug.Println("cannot set", f.Name)
			continue
		}

		var newpaths []string
		var haveset bool
		// deal with colon paths
		for _, path := range filepath.SplitList(fv.String()) {
			if strings.HasPrefix(path, old) {
				haveset = true
				newpaths = append(newpaths, new+strings.TrimPrefix(path, old))
			} else {
				newpaths = append(newpaths, path)
			}
		}

		if haveset {
			fv.SetString(strings.Join(newpaths, string(filepath.ListSeparator)))
		}
	}

	return
}
