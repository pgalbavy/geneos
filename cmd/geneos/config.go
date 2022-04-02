package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

func init() {
	RegsiterCommand(Command{
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

	RegsiterCommand(Command{
		Name:          "rename",
		Function:      commandRename,
		ParseFlags:    defaultFlag,
		ParseArgs:     parseArgs,
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
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   `geneos delete [-f] [TYPE] [NAME...]`,
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
	deleteFlags.BoolVar(&deleteForced, "f", false, "Force delete of instances")
	deleteFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	RegsiterCommand(Command{
		Name:          "rebuild",
		Function:      commandRebuild,
		ParseFlags:    rebuildFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   `geneos rebuild [-f] [-r] [TYPE] {NAME...]`,
		Summary:       `Rebuild instance configuration files`,
		Description: `Rebuild instance configuration files based on current templates and instance configuration values
		
FLAGS:

	-f	Force rebuild for instances marked 'initial' even if configuration is not 'always'
		- 'never' is never rebuilt
	-r	restart any of instances where the configuration has changed.
		This is not normally required as both Sans and Gateways can be set to auto-reload on a time.`,
	})

	rebuildFlags = flag.NewFlagSet("rebuild", flag.ExitOnError)
	rebuildFlags.BoolVar(&rebuildForced, "f", false, "Force rebuild")
	rebuildFlags.BoolVar(&rebuildRestart, "r", false, "Restart instances after rebuild")
}

var deleteForced bool

var rebuildFlags *flag.FlagSet
var rebuildForced, rebuildRestart bool

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
func loadConfig(c Instances) (err error) {
	j := InstanceFileWithExt(c, "json")

	if err = c.Remote().readConfigFile(j, &c); err == nil {
		// return if no error, else drop through
		return
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

	// read the cofig into a struct then print it out again,
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
		for _, i := range ct.instanceMatches(name) {
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

func (r *Remotes) readConfigFile(file string, config interface{}) (err error) {
	var jsonFile []byte
	if jsonFile, err = r.readFile(file); err != nil {
		return
	}
	// dec := json.NewDecoder(jsonFile)
	return json.Unmarshal(jsonFile, &config)
}

func commandRename(ct Component, args []string, params []string) (err error) {
	var stopped, done bool
	if ct == None || len(args) != 2 {
		return ErrInvalidArgs
	}

	oldname := args[0]
	newname := args[1]

	logDebug.Println("rename", ct, oldname, newname)
	oldconf, err := ct.GetInstance(oldname)
	if err != nil {
		return fmt.Errorf("%s %s not found", ct, oldname)
	}
	if err = migrateConfig(oldconf); err != nil {
		return fmt.Errorf("%s %s cannot be migrated to new configuration format", ct, oldname)
	}

	newconf, err := ct.GetInstance(newname)
	newname, _ = splitInstanceName(newname)

	if err == nil {
		return fmt.Errorf("%s already exists", newconf)
	}

	if _, err = findInstancePID(oldconf); err != ErrProcNotExist {
		if err = stopInstance(oldconf, nil); err == nil {
			// cannot use defer startInstance() here as we have
			// not yet created the new instance
			stopped = true
			defer func() {
				if !done {
					startInstance(oldconf, nil)
				}
			}()
		} else {
			return fmt.Errorf("cannot stop %s", oldname)
		}
	}

	// now a full clean
	if err = oldconf.Clean(true, []string{}); err != nil {
		return
	}

	// move directory
	if err = copyTree(oldconf.Remote(), oldconf.Home(), newconf.Remote(), newconf.Home()); err != nil {
		return
	}

	// delete one or the other, depending
	defer func() {
		if done {
			// once we are done, try to delete old instance
			oldold, _ := ct.GetInstance(oldname)
			logDebug.Println("removing old instance", oldold)
			oldold.Remote().removeAll(oldold.Home())
			log.Println(ct, oldold, "moved to", newconf)
		} else {
			// remove new instance
			logDebug.Println("removing new instance", newconf)
			newconf.Remote().removeAll(newconf.Home())
		}
	}()

	// update oldconf here and then write that out as if it were newconf
	// this gets around the defaults set in newconf being incomplete and wrong
	if err = changeDirPrefix(oldconf, oldconf.Remote().GeneosRoot(), newconf.Remote().GeneosRoot()); err != nil {
		return
	}

	// update *Home manually, as it's not just the prefix
	if err = setField(oldconf, oldconf.Prefix("Home"), filepath.Join(newconf.Type().ComponentDir(newconf.Remote()), newname)); err != nil {
		return
	}

	// after path updates, rename non paths
	ib := oldconf.Base()
	ib.InstanceLocation = newconf.Remote().RemoteName()
	ib.InstanceRemote = newconf.Remote()
	ib.InstanceName = newname

	// update any component name only if the same as the instance name
	if getString(oldconf, oldconf.Prefix("Name")) == oldname {
		if err = setField(oldconf, oldconf.Prefix("Name"), newname); err != nil {
			return
		}
	}

	// config changes don't matter until writing config succeeds
	if err = writeInstanceConfig(oldconf); err != nil {
		return
	}

	//	oldconf.Unload()
	if err = oldconf.Rebuild(false); err != nil && err != ErrNotSupported {
		return
	}

	done = true
	if stopped {
		return startInstance(oldconf, nil)
	}
	return nil
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
		if err = c.Remote().removeAll(c.Home()); err != nil {
			return
		}
		c.Unload()
		log.Println(c, "deleted")
		return nil
	}

	log.Println(c, "must use -f or instance must be be disabled before delete")
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
	if !rebuildRestart {
		return
	}
	return restartInstance(c, params)
}
