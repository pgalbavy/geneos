package main

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
)

func init() {
	RegisterCommand(Command{
		Name:          "show",
		Function:      commandShow,
		ParseFlags:    defaultFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine: `geneos show
	geneos show [global|user]
	geneos show [TYPE] [NAME...]`,
		Summary: `Show runtime, global, user or instance configuration is JSON format`,
		Description: `Show the runtime, global, user or instance configuration.

With no arguments show the resolved runtime configuration that
results from environment variables, loading built-in defaults and the
global and user configurations.

If the liternal keyword 'global' or 'user' is supplied then any
on-disk configuration for the respective options will be shown.

If a component TYPE and/or instance NAME(s) are supplied then the
configuration for those instances are output as JSON. This is
regardless of the instance using a legacy .rc file or a native JSON
configuration.

Passwords and secrets are redacted in a very simplistic manner simply
to prevent visibility in casual viewing.`})

	RegisterCommand(Command{
		Name:          "delete",
		Function:      commandDelete,
		ParseFlags:    deleteFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   `geneos delete [-F] [TYPE] [NAME...]`,
		Summary:       `Delete an instance. Instance must be stopped.`,
		Description: `Delete the matching instances. This will only work on instances that
are disabled to prevent accidental deletion. The instance directory
is removed without being backed-up. The user running the command must
have the appropriate permissions and a partial deletion cannot be
protected against.

FLAGS:

	-f	Force deletion of matching instances. Be careful, and instead disable instances
		and use a normal delete command.`,
	})

	deleteFlags = flag.NewFlagSet("delete", flag.ExitOnError)
	deleteFlags.BoolVar(&deleteForced, "F", false, "Force delete of instances")
	deleteFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	RegisterCommand(Command{
		Name:          "rebuild",
		Function:      commandRebuild,
		ParseFlags:    rebuildFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   `geneos rebuild [-F] [-r] [TYPE] {NAME...]`,
		Summary:       `Rebuild instance configuration files`,
		Description: `Rebuild instance configuration files based on current templates and instance configuration values
		
FLAGS:

	-f	Force rebuild for instances marked 'initial' even if configuration is not 'always'
		- 'never' is never rebuilt
	-r	reload any of instances where the configuration has changed.
		This is not normally required as both Sans and Gateways can be set to auto-reload on a time.`,
	})

	rebuildFlags = flag.NewFlagSet("rebuild", flag.ExitOnError)
	rebuildFlags.BoolVar(&rebuildForced, "F", false, "Force rebuild")
	rebuildFlags.BoolVar(&rebuildReload, "r", false, "Reload instances after rebuild")
}

var deleteForced bool

var rebuildFlags *flag.FlagSet
var rebuildForced, rebuildReload bool

func deleteFlag(command string, args []string) []string {
	deleteFlags.Parse(args)
	checkHelpFlag(command)
	return deleteFlags.Args()
}

func rebuildFlag(command string, args []string) []string {
	rebuildFlags.Parse(args)
	checkHelpFlag(command)
	return rebuildFlags.Args()
}

// load system config from global and user JSON files and process any
// environment variables we choose
func loadSysConfig() {
	// a failure is ignored
	readLocalConfigFile(globalConfig, &GlobalConfig)

	// root should not have a per-user config, but if run by sudo the
	// HOME dir is conserved, so allow for now
	userConfDir, _ := os.UserConfigDir()
	err := readLocalConfigFile(filepath.Join(userConfDir, "geneos.json"), &GlobalConfig)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Println("reading user configuration:", err)
	}

	// setting the environment variable - to match legacy programs - overrides
	// all others

	// workaround breaking change
	if oldhome, ok := GlobalConfig["ITRSHome"]; ok {
		if newhome, ok := GlobalConfig["Geneos"]; !ok || newhome == "" {
			GlobalConfig["Geneos"] = oldhome
		}
		delete(GlobalConfig, "ITRSHome")
	}

	// env variable overrides all other settings
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		GlobalConfig["Geneos"] = h
	}
}

func Geneos() string {
	home, ok := GlobalConfig["Geneos"]
	if !ok {
		// fallback to support braking change
		home, ok = GlobalConfig["ITRSHome"]
		if !ok {
			return ""
		}
	}
	return home
}

// loadConfig will load the JSON config file is available, otherwise
// try to load the "legacy" .rc file
//
// support cache?
//
// error check core values - e.g. Name
func loadConfig(c Instances) (err error) {
	j := InstanceFileWithExt(c, "json")

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

func commandShow(ct Component, args []string, params []string) (err error) {
	// default to combined global + user config
	// allow overrides to show specific or components
	if len(args) == 0 && ct == None {
		// special case "config show" for resolved settings
		printConfigJSON(GlobalConfig)
		return
	}

	// read the config into a struct then print it out again,
	// to sanitise the contents - or generate an error
	if len(args) > 0 {
		switch args[0] {
		case "global":
			var c GlobalSettings
			readLocalConfigFile(globalConfig, &c)
			printConfigJSON(c)
			return
		case "user":
			var c GlobalSettings
			userConfDir, _ := os.UserConfigDir()
			readLocalConfigFile(filepath.Join(userConfDir, "geneos.json"), &c)
			printConfigJSON(c)
			return
		}
	}

	// need this to process @remote args etc.
	_, args, _ = parseArgs(commands["show"], args)

	// loop instances - parse the args again and load/print the config,
	// but allow for RC files again
	//
	cs := make(map[RemoteName][]Instances)
	for _, name := range args {
		for _, i := range ct.FindInstances(name) {
			cs[i.Remote().RemoteName()] = append(cs[i.Remote().RemoteName()], i)
		}
	}
	if len(cs) > 0 {
		printConfigJSON(cs)
		return
	}

	log.Println("no matches to show")

	return
}

func printConfigJSON(Config interface{}) (err error) {
	var buffer []byte
	if buffer, err = json.MarshalIndent(Config, "", "    "); err != nil {
		return
	}
	j := string(buffer)
	j = opaqueJSONSecrets(j)
	log.Printf("%s\n", j)
	return
}

// XXX redact passwords - any field matching some regexp ?
// also embedded Envs
//
//
var red1 = regexp.MustCompile(`"(.*((?i)pass|password|secret))": "(.*)"`)
var red2 = regexp.MustCompile(`"(.*((?i)pass|password|secret))=(.*)"`)

func opaqueJSONSecrets(j string) string {
	// simple redact - and left field with "Pass" in it gets the right replaced
	j = red1.ReplaceAllString(j, `"$1": "********"`)
	j = red2.ReplaceAllString(j, `"$1=********"`)
	return j
}

func readLocalConfigFile(file string, config interface{}) (err error) {
	jsonFile, err := os.ReadFile(file)
	if err != nil {
		return
	}
	// dec := json.NewDecoder(jsonFile)
	return json.Unmarshal(jsonFile, &config)
}

func (r *Remotes) readConfigFile(file string, config interface{}) (jsonFile []byte, err error) {
	if jsonFile, err = r.ReadFile(file); err != nil {
		return
	}
	// dec := json.NewDecoder(jsonFile)
	return jsonFile, json.Unmarshal(jsonFile, &config)
}

func commandDelete(ct Component, args []string, params []string) (err error) {
	return ct.loopCommand(deleteInstance, args, params)
}

func deleteInstance(c Instances, params []string) (err error) {
	if deleteForced {
		if components[c.Type()].RealComponent {
			if err = stopInstance(c, nil); err != nil {
				return
			}
		}
	}

	if deleteForced || Disabled(c) {
		if err = c.Remote().RemoveAll(c.Home()); err != nil {
			return
		}
		log.Printf("%s deleted %s:%s", c, c.Remote().InstanceName, c.Home())
		c.Unload()
		return nil
	}

	log.Println(c, "must use -F or instance must be be disabled before delete")
	return nil
}

func commandRebuild(ct Component, args []string, params []string) (err error) {
	return ct.loopCommand(rebuildInstance, args, params)
}

func rebuildInstance(c Instances, params []string) (err error) {
	if err = c.Rebuild(rebuildForced); err != nil {
		if err == ErrNoAction {
			err = nil
		}
		return
	}
	log.Println(c, "configuration rebuilt")
	if !rebuildReload {
		return
	}
	return reloadInstance(c, params)
}
