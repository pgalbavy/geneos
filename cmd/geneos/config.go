package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func init() {
	RegsiterCommand(Command{
		Name:          "show",
		Function:      commandShow,
		ParseFlags:    defaultFlag,
		ParseArgs:     defaultArgs,
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

	RegsiterCommand(Command{
		Name:          "rename",
		Function:      commandRename,
		ParseFlags:    defaultFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: true,
		CommandLine:   `geneos rename TYPE FROM TO`,
		Summary:       `Rename an instance`,
		Description: `Rename an instance. TYPE is requied to resolve any ambiguities if two instances
share the same name. No configuration changes outside the instance JSON config file. As
any existing .rc legacy file is never changed, this will migrate the instance from .rc to JSON.
The instance is stopped and restarted after the instance directory and configuration are changed.
It is an error to try to rename an instance to one that already exists with the same name.`,
	})

	RegsiterCommand(Command{
		Name:          "delete",
		Function:      commandDelete,
		ParseFlags:    deleteFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   `geneos delete [TYPE] [NAME...]`,
		Summary:       `Delete an instance. Instance must be stopped.`,
		Description: `Delete the matching instances. This will only work on instances that are disabled to prevent
accidental deletion. The instance directory is removed without being backed-up. The user running
the command must have the appropriate permissions and a partial deletion cannot be protected
against.`,
	})

	deleteFlags = flag.NewFlagSet("delete", flag.ExitOnError)
	deleteFlags.BoolVar(&deleteForced, "f", false, "Override need to have disabled instances")
	deleteFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	RegsiterCommand(Command{
		Name:          "rebuild",
		Function:      commandRebuild,
		ParseFlags:    rebuildFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   `geneos rebuild [-f] [-n] [TYPE] {NAME...]`,
		Summary:       `Rebuild instance configuration files`,
		Description: `Rebuild instance configuration files based on current templates and instance configuration values
		
FLAGS:

	-f	Force rebuild for instances marked 'initial' even if configuration is not 'always' - 'never' is never rebuilt
	-n	No restart of instances`,
	})

	rebuildFlags = flag.NewFlagSet("rebuild", flag.ExitOnError)
	rebuildFlags.BoolVar(&rebuildForced, "f", false, "Force rebuild")
	rebuildFlags.BoolVar(&rebuildNoRestart, "n", false, "Do not restart instances after rebuild")
}

var deleteForced bool

var rebuildFlags *flag.FlagSet
var rebuildForced, rebuildNoRestart bool

var globalConfig = "/etc/geneos/geneos.json"

var initDirs = []string{}

// new global config
type Global string
type GlobalSettings map[Global]string

var GlobalConfig GlobalSettings = make(GlobalSettings)

// load system config from global and user JSON files and process any
// environment variables we choose
func loadSysConfig() {
	readLocalConfigFile(globalConfig, &GlobalConfig)

	// root should not have a per-user config, but if sun by sudo the
	// HOME dir is conserved, so allow for now
	userConfDir, _ := os.UserConfigDir()
	err := readLocalConfigFile(filepath.Join(userConfDir, "geneos.json"), &GlobalConfig)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Println(err)
	}

	// setting the environment variable - to match legacy programs - overrides
	// all others
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		GlobalConfig["ITRSHome"] = h
	}
}

func ITRSHome() string {
	home, ok := GlobalConfig["ITRSHome"]
	if !ok {
		return ""
	}
	return home
}

// loadConfig will load the JSON config file is available, otherwise
// try to load the "legacy" .rc file
//
// support cache?
func loadConfig(c Instances) (err error) {
	baseconf := filepath.Join(c.Home(), c.Type().String())
	j := baseconf + ".json"

	if err = c.Remote().readConfigFile(j, &c); err == nil {
		// return if no error, else drop through
		return
	}
	if err = readRCConfig(c); err != nil {
		return
	}
	return
}

func commandShow(ct Component, names []string, params []string) (err error) {
	// default to combined global + user config
	// allow overrides to show specific or components
	if len(names) == 0 && ct == None {
		// special case "config show" for resolved settings
		printConfigStructJSON(GlobalConfig)
		return
	}

	// read the cofig into a struct then print it out again,
	// to sanitise the contents - or generate an error
	if len(names) > 0 {
		switch names[0] {
		case "global":
			var c GlobalSettings
			readLocalConfigFile(globalConfig, &c)
			printConfigStructJSON(c)
			return
		case "user":
			var c GlobalSettings
			userConfDir, _ := os.UserConfigDir()
			readLocalConfigFile(filepath.Join(userConfDir, "geneos.json"), &c)
			printConfigStructJSON(c)
			return
		}
	}

	// loop instances - parse the args again and load/print the config,
	// but allow for RC files again
	var cs []Instances
	for _, name := range names {
		cs = append(cs, ct.instanceMatches(name)...)
	}
	if len(cs) > 0 {
		printConfigSliceJSON(cs)
		return
	}

	log.Println("no matches to show")

	return
}

// given a slice of structs, output as a JSON array of maps
func printConfigSliceJSON(Slice []Instances) {
	js := []string{}

	for _, i := range Slice {
		x, err := marshalStruct(i, "    ")
		if err != nil {
			// recover later
			logError.Fatalln(err)
		}
		js = append(js, x)

	}
	s := "[\n    " + strings.Join(js, ",\n    ") + "\n]"
	log.Println(s)
}

func printConfigStructJSON(Config interface{}) (err error) {
	if j, err := marshalStruct(Config, ""); err == nil {
		log.Printf("%s\n", j)
	}
	return
}

// XXX redact passwords - any field matching some regexp ?
// also embedded Envs
//
//
var red1 = regexp.MustCompile(`"(.*((?i)pass|password|secret))": "(.*)"`)
var red2 = regexp.MustCompile(`"(.*((?i)pass|password|secret))=(.*)"`)

func marshalStruct(s interface{}, prefix string) (j string, err error) {
	if buffer, err := json.MarshalIndent(s, prefix, "    "); err == nil {
		j = string(buffer)
	}
	// simple redact - and left field with "Pass" in it gets the right replaced
	j = red1.ReplaceAllString(j, `"$1": "********"`)
	j = red2.ReplaceAllString(j, `"$1=********"`)
	return
}

func readLocalConfigFile(file string, config interface{}) (err error) {
	jsonFile, err := os.ReadFile(file)
	if err != nil {
		return
	}
	// dec := json.NewDecoder(jsonFile)
	return json.Unmarshal(jsonFile, &config)
}

func (r *Remotes) readConfigFile(file string, config interface{}) (err error) {
	jsonFile, err := r.readFile(file)
	if err != nil {
		return
	}
	// dec := json.NewDecoder(jsonFile)
	return json.Unmarshal(jsonFile, &config)
}

func commandRename(ct Component, args []string, params []string) (err error) {
	if ct == None || len(args) != 2 {
		return ErrInvalidArgs
	}

	oldname := args[0]
	newname := args[1]

	logDebug.Println("rename", ct, oldname, newname)
	oldconf, err := ct.getInstance(oldname)
	if err != nil {
		return fmt.Errorf("%s %s not found", ct, oldname)
	}
	if err = migrateConfig(oldconf); err != nil {
		return fmt.Errorf("%s %s cannot be migrated to new configuration format", ct, oldname)
	}
	newconf, err := ct.getInstance(newname)
	if err == nil {
		return fmt.Errorf("%s %s already exists", ct, newname)
	}

	stopInstance(oldconf, nil)

	// save for recover, as config gets changed
	oldhome := oldconf.Home()
	newhome := newconf.Home()

	if err = oldconf.Remote().renameFile(oldhome, newhome); err != nil {
		logDebug.Println("rename failed:", oldhome, newhome, err)
		return
	}

	// rename fields

	if err = setField(oldconf, "InstanceName", newname); err != nil {
		// try to recover
		_ = newconf.Remote().renameFile(newhome, oldhome)
		return
	}
	// update any component name if the same as the instance name
	if getString(oldconf, oldconf.Prefix("Name")) == oldname {
		if err = setField(oldconf, oldconf.Prefix("Name"), newname); err != nil {
			// try to recover
			_ = newconf.Remote().renameFile(newhome, oldhome)
			return
		}
	}

	if err = setField(oldconf, oldconf.Prefix("Home"), filepath.Join(ct.componentDir(newconf.Location()), newname)); err != nil {
		// try to recover
		_ = newconf.Remote().renameFile(newhome, oldhome)
		return
	}

	// config changes don't matter until writing config succeeds
	if err = newconf.Remote().writeConfigFile(filepath.Join(newhome, ct.String()+".json"), newconf.Prefix("User"), oldconf); err != nil {
		_ = newconf.Remote().renameFile(newhome, oldhome)
		return
	}
	log.Println(ct, oldname, "renamed to", newname)
	return startInstance(oldconf, nil)
}

func commandDelete(ct Component, args []string, params []string) (err error) {
	return ct.loopCommand(deleteInstance, args, params)
}

func deleteInstance(c Instances, params []string) (err error) {
	if deleteForced {
		if components[c.Type()].RealComponent {
			stopInstance(c, nil)
		}
		if err = c.Remote().removeAll(c.Home()); err != nil {
			logError.Fatalln(err)
		}
		return nil
	}

	if Disabled(c) {
		if err = c.Remote().removeAll(c.Home()); err != nil {
			logError.Fatalln(err)
		}
		return nil
	}
	log.Println(c.Type(), c.Name(), "must be disabled before delete")
	return nil
}

func deleteFlag(command string, args []string) []string {
	deleteFlags.Parse(args)
	checkHelpFlag(command)
	return deleteFlags.Args()
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
	if rebuildNoRestart {
		return
	}
	return restartInstance(c, params)
}

func rebuildFlag(command string, args []string) []string {
	rebuildFlags.Parse(args)
	checkHelpFlag(command)
	return rebuildFlags.Args()
}
