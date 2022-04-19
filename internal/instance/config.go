package instance

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"text/template"

	"wonderland.org/geneos/internal/geneos"
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
func CreateConfigFromTemplate(c geneos.Instance, path string, name string, defaultTemplate []byte) (err error) {
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
func LoadConfig(c geneos.Instance) (err error) {
	j := ConfigPathWithExt(c, "json")

	var n Instance
	var jsonFile []byte
	if jsonFile, err = c.Remote().ReadConfigFile(j, &n); err == nil {
		// validate base here
		if c.Name() != n.InstanceName {
			logError.Println(c, "inconsistent configuration file contents:", j)
			return geneos.ErrInvalidArgs
		}
		//if we validate then Unmarshal same file over existing instance
		if err = json.Unmarshal(jsonFile, &c); err == nil {
			if c.Home() != filepath.Dir(j) {
				logError.Printf("%s has a configured home directory different to real location: %q != %q", c, filepath.Dir(j), c.Home())
				return geneos.ErrInvalidArgs
			}
			// return if no error, else drop through
			return
		}
	}

	return readRCConfig(c)
}

// read an old style .rc file. parameters are one-per-line and are key=value
// any keys that do not match the component prefix or the special
// 'BinSuffix' are treated as environment variables
//
// No processing of shell variables. should there be?
func readRCConfig(c geneos.Instance) (err error) {
	rcdata, err := c.Remote().ReadFile(ConfigPathWithExt(c, "rc"))
	if err != nil {
		return
	}
	logDebug.Printf("loading config from %s/%s.rc", c.Home(), c.Type())

	confs := make(map[string]string)

	rcFile := bytes.NewBuffer(rcdata)
	scanner := bufio.NewScanner(rcFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		s := strings.SplitN(line, "=", 2)
		if len(s) != 2 {
			return fmt.Errorf("invalid line (must be key=value) %q: %w", line, geneos.ErrInvalidArgs)
		}
		key, value := s[0], s[1]
		value = strings.Trim(value, "\"")
		confs[key] = value
	}

	var env []string
	for k, v := range confs {
		if k == "BinSuffix" {
			c.V().Set(k, v)
			continue
		}
		// this doesn't work if Prefix is empty
		if strings.HasPrefix(k, c.Prefix("")) {
			c.V().Set(k, v)
		} else {
			// set env var
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	c.V().Set("Env", env)
	return
}

func ConfigPathWithExt(c geneos.Instance, extension string) (path string) {
	return filepath.Join(c.Home(), c.Type().String()+"."+extension)
}

// check for rc file? migrate?
// func writeInstanceConfig(c geneos.Instance) (err error) {
// 	return c.Remote().writeConfigFile(ConfigPathWithExt(c, "json"), c.Prefix("User"), 0664, c)
// }

func WriteConfig(c geneos.Instance) (err error) {
	file := ConfigPathWithExt(c, "json")
	return c.V().WriteConfigAs(file)
}

func ReadConfig(c geneos.Instance) (err error) {
	file := ConfigPathWithExt(c, "json")
	logDebug.Println(file)
	c.V().SetConfigFile(file)
	return c.V().MergeInConfig()
}

// migrate config from .rc to .json, but check first
func Migrate(c geneos.Instance) (err error) {
	// if no .rc, return
	if _, err = c.Remote().Stat(ConfigPathWithExt(c, "rc")); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	// if .json exists, return
	if _, err = c.Remote().Stat(ConfigPathWithExt(c, "json")); err == nil {
		return nil
	}

	// write new .json
	if err = WriteConfig(c); err != nil {
		logError.Println("failed to write config file:", err)
		return
	}

	// back-up .rc
	if err = c.Remote().Rename(ConfigPathWithExt(c, "rc"), ConfigPathWithExt(c, "rc.orig")); err != nil {
		logError.Println("failed to rename old config:", err)
	}

	logDebug.Printf("migrated %s to JSON config", c)
	return
}
