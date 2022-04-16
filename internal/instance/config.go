package instance

import (
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"text/template"
)

// return the KEY from "[TYPE:]KEY=VALUE"
func keyOf(s string, sep string) string {
	r := strings.SplitN(s, sep, 2)
	return r[0]
}

// return the VALUE from "[TYPE:]KEY=VALUE"
func valueOf(s string, sep string) string {
	r := strings.SplitN(s, sep, 2)
	if len(r) > 0 {
		return r[1]
	}
	return ""
}

func first(d ...string) string {
	for _, s := range d {
		if s != "" {
			return s
		}
	}
	return ""
}

var fnmap template.FuncMap = template.FuncMap{
	"first":   first,
	"join":    filepath.Join,
	"keyOf":   keyOf,
	"valueOf": valueOf,
}

//
// load templates from TYPE/templates/[tmpl]* and parse it using the instance data
// write it out to a single file. If tmpl is empty, load all files
//
func createConfigFromTemplate(c Instance, path string, name string, defaultTemplate []byte) (err error) {
	var out io.WriteCloser
	// var t *template.Template

	t := template.New("").Funcs(fnmap)
	if t, err = t.ParseGlob(c.Remote().GeneosPath(c.Type().String(), "templates", "*")); err != nil {
		// if there are no templates, use internal as a fallback
		log.Printf("No templates found in %s, using internal defaults", c.Remote().GeneosPath(c.Type().String(), "templates"))
		t = template.Must(t.Parse(string(defaultTemplate)))
	}

	// XXX backup old file - use same scheme as writeConfigFile()

	if out, err = c.Remote().Create(path, 0660); err != nil {
		log.Printf("Cannot create configuration file for %s %s", c, path)
		return err
	}
	defer out.Close()

	if err = t.ExecuteTemplate(out, name, c); err != nil {
		log.Println("Cannot create configuration from template(s):", err)
		return err
	}

	return
}

// loadConfig will load the JSON config file is available, otherwise
// try to load the "legacy" .rc file
//
// support cache?
//
// error check core values - e.g. Name
func LoadConfig(c Instance) (err error) {
	j := ConfigPathWithExt(c, "json")

	var n InstanceBase
	var jsonFile []byte
	if jsonFile, err = c.Remote().readConfigFile(j, &n); err == nil {
		// validate base here
		if c.Name() != n.InstanceName {
			logError.Println(c, "inconsistent configuration file contents:", j)
			return ErrInvalidArgs
		}
		//if we validate then Unmarshal same file over existing instance
		if err = json.Unmarshal(jsonFile, &c); err == nil {
			if c.Home() != filepath.Dir(j) {
				logError.Printf("%s has a configured home directory different to real location: %q != %q", c, filepath.Dir(j), c.Home())
				return ErrInvalidArgs
			}
			// return if no error, else drop through
			return
		}
	}

	return readRCConfig(c)
}

func ConfigPathWithExt(c Instance, extension string) (path string) {
	return filepath.Join(c.Home(), c.Type().String()+"."+extension)
}

// check for rc file? migrate?
func writeInstanceConfig(c Instance) (err error) {
	return c.Remote().writeConfigFile(instance.ConfigPathWithExt(c, "json"), c.Prefix("User"), 0664, c)
}
