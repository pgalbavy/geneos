package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"text/template"
)

// reflect methods to get and set struct fields

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
		default:
			return fmt.Errorf("cannot set %q to a %T: %w", k, v, ErrInvalidArgs)
		}
	} else {
		return fmt.Errorf("cannot set %q: %w (isValid=%v, canset=%v, type=%v)", k, ErrInvalidArgs, fv.IsValid(), fv.CanSet(), fv.Type())
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