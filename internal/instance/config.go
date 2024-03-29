package instance

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/spf13/afero/sftpfs"
	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
)

type ExtraConfigValues struct {
	Includes   IncludeValues
	Gateways   GatewayValues
	Attributes StringSliceValues
	Envs       StringSliceValues
	Variables  VarValues
	Types      StringSliceValues
	Keys       StringSliceValues
}

// return the KEY from "[TYPE:]KEY=VALUE"
func nameOf(s string, sep string) string {
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

func first(d ...interface{}) string {
	for _, f := range d {
		if s, ok := f.(string); ok {
			if s != "" {
				return s
			}
		}
	}
	return ""
}

var fnmap template.FuncMap = template.FuncMap{
	"first":   first,
	"join":    filepath.Join,
	"nameOf":  nameOf,
	"valueOf": valueOf,
}

//
// load templates from TYPE/templates/[tmpl]* and parse it using the instance data
// write it out to a single file. If tmpl is empty, load all files
//
func CreateConfigFromTemplate(c geneos.Instance, path string, name string, defaultTemplate []byte) (err error) {
	var out io.WriteCloser
	// var t *template.Template

	t := template.New("").Funcs(fnmap).Option("missingkey=zero")
	if t, err = t.ParseGlob(c.Host().GeneosJoinPath(c.Type().String(), "templates", "*")); err != nil {
		t = template.New(name).Funcs(fnmap).Option("missingkey=zero")
		// if there are no templates, use internal as a fallback
		log.Printf("No templates found in %s, using internal defaults", c.Host().GeneosJoinPath(c.Type().String(), "templates"))
		t = template.Must(t.Parse(string(defaultTemplate)))
	}

	// XXX backup old file - use same scheme as writeConfigFile()

	if out, err = c.Host().Create(path, 0660); err != nil {
		log.Printf("Cannot create configuration file for %s %s", c, path)
		return err
	}
	defer out.Close()

	// m := make(map[string]string)
	m := c.V().AllSettings()
	// viper insists this is a float64, manually override
	m["port"] = uint16(c.V().GetUint("port"))
	// set high level defaults
	m["root"] = c.Host().GetString("geneos")
	m["name"] = c.Name()
	// XXX remove aliases ??
	for _, k := range c.V().AllKeys() {
		if _, ok := c.Type().Aliases[k]; ok {
			delete(m, k)
		}
	}
	logDebug.Printf("template data: %#v", m)

	if err = t.ExecuteTemplate(out, name, m); err != nil {
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
	if c.Host().Failed() {
		return
	}
	if err = ReadConfig(c); err == nil {
		return
	}

	err = readRCConfig(c)
	if err != nil {
		// generic error as no .json or .rc found
		return fmt.Errorf("no configuration files for %s in %s: %w", c, c.Home(), os.ErrNotExist)
	}
	return
}

// read an old style .rc file. parameters are one-per-line and are key=value
// any keys that do not match the component prefix or the special
// 'BinSuffix' are treated as environment variables
//
// No processing of shell variables. should there be?
func readRCConfig(c geneos.Instance) (err error) {
	rcdata, err := c.Host().ReadFile(ConfigPathWithExt(c, "rc"))
	if err != nil {
		return
	}
	logDebug.Printf("loading config from %q", ConfigPathWithExt(c, "rc"))

	confs := make(map[string]string)

	scanner := bufio.NewScanner(bytes.NewBuffer(rcdata))
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
		// trim double and single quotes and tabs and spaces from value
		value = strings.Trim(value, "\"' \t")
		confs[key] = value
	}

	var env []string
	for k, v := range confs {
		if strings.Contains(v, "${") {
			v = evalOldVars(c, v)
		}
		lk := strings.ToLower(k)
		if lk == "binary" {
			c.V().Set(k, v)
			continue
		}
		if strings.HasPrefix(lk, c.Prefix()) {
			nk := c.Type().Aliases[lk]
			c.V().Set(nk, v)
		} else {
			// set env var
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	if len(env) > 0 {
		c.V().Set("Env", env)
	}
	return
}

// first expand against viper, then env
func evalOldVars(c geneos.Instance, in string) (out string) {
	out = in

	// replace aliases in output with kew keys
	for k, v := range c.Type().Aliases {
		// aliases are hardcoded, they will compile
		// we could cache these, but it's probably not worth it
		re := regexp.MustCompile(`(?i)\$\{` + k + `\}`)
		out = re.ReplaceAllString(out, `$${`+v+"}")
	}

	// replace resulting keys with values
	for _, k := range c.V().AllKeys() {
		out = strings.ReplaceAll(out, "${"+k+"}", c.V().GetString(k))
	}

	// finally expand env vars
	out = os.ExpandEnv(out)
	return
}

func ConfigPathWithExt(c geneos.Instance, extension string) (path string) {
	return filepath.Join(c.Home(), c.Type().String()+"."+extension)
}

// write out an instance configuration file.
// XXX check if existing config is an .rc file and if so rename it after
// successful write to match migrate
//
// remote configuration files are supported using afero.Fs through
// viper but rely on host.DialSFTP to dial and cache the client
//
// delete any aliases fields before writing
func WriteConfig(c geneos.Instance) (err error) {
	file := ConfigPathWithExt(c, "json")
	if err = c.Host().MkdirAll(filepath.Dir(file), 0775); err != nil {
		logError.Println(err)
	}
	nv := viper.New()
	for _, k := range c.V().AllKeys() {
		if _, ok := c.Type().Aliases[k]; !ok {
			nv.Set(k, c.V().Get(k))
		}
	}
	if c.Host() != host.LOCAL {
		client, err := c.Host().DialSFTP()
		if err != nil {
			logError.Println(err)
		}
		nv.SetFs(sftpfs.New(client))
	}
	logDebug.Printf("writing config for %s as %q", c, file)
	return nv.WriteConfigAs(file)
}

func WriteConfigValues(c geneos.Instance, values map[string]interface{}) error {
	file := ConfigPathWithExt(c, "json")
	nv := viper.New()
	for k, v := range values {
		nv.Set(k, v)
	}
	if c.Host() != host.LOCAL {
		client, err := c.Host().DialSFTP()
		if err != nil {
			logError.Println(err)
		}
		nv.SetFs(sftpfs.New(client))
	}
	return nv.WriteConfigAs(file)
}

func ReadConfig(c geneos.Instance) (err error) {
	c.V().AddConfigPath(c.Home())
	c.V().SetConfigName(c.Type().String())
	c.V().SetConfigType("json")
	if c.Host() != host.LOCAL {
		client, err := c.Host().DialSFTP()
		if err != nil {
			logError.Printf("connection to %s failed", c.Host())
			return err
		}
		c.V().SetFs(sftpfs.New(client))
	}
	err = c.V().MergeInConfig()

	// aliases have to be set AFTER loading from file (https://github.com/spf13/viper/issues/560)
	for a, k := range c.Type().Aliases {
		// logger.Debug.Printf("register %q as alias for %q", k, v)
		c.V().RegisterAlias(a, k)
	}
	if err == nil {
		logDebug.Printf("config loaded for %s from %q", c, c.V().ConfigFileUsed())
	}
	return
}

// migrate config from .rc to .json, but check first
func Migrate(c geneos.Instance) (err error) {
	// if no .rc, return
	if _, err = c.Host().Stat(ConfigPathWithExt(c, "rc")); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	// if .json exists, return
	if _, err = c.Host().Stat(ConfigPathWithExt(c, "json")); err == nil {
		return nil
	}

	// write new .json
	if err = WriteConfig(c); err != nil {
		logError.Println("failed to write config file:", err)
		return
	}

	// back-up .rc
	if err = c.Host().Rename(ConfigPathWithExt(c, "rc"), ConfigPathWithExt(c, "rc.orig")); err != nil {
		logError.Println("failed to rename old config:", err)
	}

	logDebug.Printf("migrated %s to JSON config", c)
	return
}

// a template function to support "{{join .X .Y}}"
var textJoinFuncs = template.FuncMap{"join": filepath.Join}

// SetDefaults() is a common function called by component factory
// functions to iterate over the component specific instance
// struct and set the defaults as defined in the 'defaults'
// struct tags.
func SetDefaults(c geneos.Instance, name string) (err error) {
	aliases := c.Type().Aliases
	c.V().SetDefault("name", name)
	if c.Type().Defaults != nil {
		// set bootstrap values used by templates
		root := c.Host().GetString("geneos")
		for _, s := range c.Type().Defaults {
			var b bytes.Buffer
			p := strings.SplitN(s, "=", 2)
			k, v := p[0], p[1]
			val, err := template.New(k).Funcs(textJoinFuncs).Parse(v)
			if err != nil {
				logError.Println(c, "parse error:", v)
				return err
			}
			if c.V() == nil {
				logError.Println("no viper found")
			}
			// add a bootstrap for 'root'
			settings := c.V().AllSettings()
			settings["root"] = root
			if err = val.Execute(&b, settings); err != nil {
				log.Println(c, "cannot set defaults:", v)
				return err
			}
			// if default is an alias, resolve it here
			if aliases != nil {
				nk, ok := aliases[k]
				if ok {
					k = nk
				}
			}
			c.V().SetDefault(k, b.String())
		}
	}

	return
}

// Value types for multiple flags

// XXX abstract this for a general case
func SetExtendedValues(c geneos.Instance, x ExtraConfigValues) (changed bool) {
	if setSlice(c, x.Attributes, "attributes", func(a string) string {
		return strings.SplitN(a, "=", 2)[0]
	}) {
		changed = true
	}

	if setSlice(c, x.Envs, "env", func(a string) string {
		return strings.SplitN(a, "=", 2)[0]
	}) {
		changed = true
	}

	if setSlice(c, x.Types, "types", func(a string) string {
		return a
	}) {
		changed = true
	}

	if len(x.Gateways) > 0 {
		gateways := c.V().GetStringMapString("gateways")
		for k, v := range x.Gateways {
			gateways[k] = v
		}
		c.V().Set("gateways", gateways)
	}

	if len(x.Includes) > 0 {
		incs := c.V().GetStringMapString("includes")
		for k, v := range x.Includes {
			incs[k] = v
		}
		c.V().Set("includes", incs)
	}

	if len(x.Variables) > 0 {
		vars := c.V().GetStringMapString("variables")
		for k, v := range x.Variables {
			vars[k] = v
		}
		c.V().Set("variables", vars)
	}

	return
}

// sets 'items' in the settings identified by 'key'. the key() function returns an identifier to use
// in merge comparisons
func setSlice(c geneos.Instance, items []string, setting string, key func(string) string) (changed bool) {
	if len(items) == 0 {
		return
	}

	newvals := []string{}
	vals := c.V().GetStringSlice(setting)

	if len(vals) == 0 {
		c.V().Set(setting, items)
		changed = true
		return
	}

	// map to store the identifier and the full value for later checks
	keys := map[string]string{}
	for _, v := range items {
		keys[key(v)] = v
		newvals = append(newvals, v)
	}

	for _, v := range vals {
		if w, ok := keys[key(v)]; ok {
			// exists
			if v != w {
				// only changed if different value
				changed = true
				continue
			}
		} else {
			// copying the old value is not a change
			newvals = append(newvals, v)
		}
	}

	// check old values against map, copy those that do not exist

	c.V().Set(setting, newvals)
	return
}

// include file - priority:url|path
type IncludeValues map[string]string

func (i *IncludeValues) String() string {
	return ""
}

func (i *IncludeValues) Set(value string) error {
	e := strings.SplitN(value, ":", 2)
	val := "100"
	if len(e) > 1 {
		val = e[1]
	} else {
		// XXX check two values and first is a number
		logDebug.Println("second value missing after ':', using default", val)
	}
	(*i)[e[0]] = val
	return nil
}

func (i *IncludeValues) Type() string {
	return "PRIORITY:{URL|PATH}"
}

// gateway - name:port
type GatewayValues map[string]string

func (i *GatewayValues) String() string {
	return ""
}

func (i *GatewayValues) Set(value string) error {
	e := strings.SplitN(value, ":", 2)
	val := "7039"
	if len(e) > 1 {
		val = e[1]
	} else {
		// XXX check two values and first is a number
		logDebug.Println("second value missing after ':', using default", val)
	}
	(*i)[e[0]] = val
	return nil
}

func (i *GatewayValues) Type() string {
	return "HOSTNAME:PORT"
}

// attribute - name=value
type StringSliceValues []string

func (i *StringSliceValues) String() string {
	return ""
}

func (i *StringSliceValues) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func (i *StringSliceValues) Type() string {
	return "NAME"
}

// variables - [TYPE:]NAME=VALUE
type VarValues map[string]string

func (i *VarValues) String() string {
	return ""
}

func (i *VarValues) Set(value string) error {
	var t, k, v string

	e := strings.SplitN(value, ":", 2)
	if len(e) == 1 {
		t = "string"
		s := strings.SplitN(e[0], "=", 2)
		k = s[0]
		if len(s) > 1 {
			v = s[1]
		}
	} else {
		t = e[0]
		s := strings.SplitN(e[1], "=", 2)
		k = s[0]
		if len(s) > 1 {
			v = s[1]
		}
	}

	// XXX check types here - e[0] options type, default string
	var validtypes map[string]string = map[string]string{
		"string":             "",
		"integer":            "",
		"double":             "",
		"boolean":            "",
		"activeTime":         "",
		"externalConfigFile": "",
	}
	if _, ok := validtypes[t]; !ok {
		logError.Printf("invalid type %q for variable", t)
		return geneos.ErrInvalidArgs
	}
	val := t + ":" + v
	(*i)[k] = val
	return nil
}

func (i *VarValues) Type() string {
	return "[TYPE:]NAME=VALUE"
}
