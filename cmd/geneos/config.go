package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func init() {
	GlobalConfig = make(GlobalSettings)

	commands["show"] = Command{
		Function:   commandShow,
		ParseFlags: defaultFlag,
		ParseArgs:  defaultArgs,
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
to prevent visibility in casual viewing.`}

	commands["set"] = Command{
		Function:   commandSet,
		ParseFlags: defaultFlag,
		ParseArgs:  defaultArgs,
		CommandLine: `geneos set [global|user] KEY=VALUE [KEY=VALUE...]
	geneos set [TYPE] [NAME...] KEY=VALUE [KEY=VALUE...]`,
		Summary: `Set runtime, global, user or instance configuration parameters`,
		Description: `Set configuration item values in global, user, or for a specific
instance.

To set enironment variables for an instance use the key Env and the
value var=value. Each new var=value is additive or overwrites an existing
entry for 'var', e.g.

	geneos set netprobe localhost Env=JAVA_HOME=/usr/lib/jre
	geneos set netprobe localhost Env=ORACLE_HOME=/opt/oracle

To remove an environment variable prefix the name with a hyphen '-', e.g.

	geneos set netprobe localhost Env=-JAVA_HOME

To add an include file to an auto-generated gateway use a similar syntax to the above, but in the form:

	geneos set gateway gateway1 Includes=100:path/to/include.xml
	geneos set gateway gateway1 Includes=-100

Then rebuild the configuration as required.

`}

	commands["rename"] = Command{
		Function:    commandRename,
		ParseFlags:  defaultFlag,
		ParseArgs:   checkComponentArg,
		CommandLine: `geneos rename TYPE FROM TO`,
		Summary:     `Rename an instance`,
		Description: `Rename an instance. TYPE is requied to resolve any ambiguities if two instances
share the same name. No configuration changes outside the instance JSON config file. As
any existing .rc legacy file is never changed, this will migrate the instance from .rc to JSON.
The instance is stopped and restarted after the instance directory and configuration are changed.
It is an error to try to rename an instance to one that already exists with the same name.`}

	commands["delete"] = Command{
		Function:    commandDelete,
		ParseFlags:  deleteFlag,
		ParseArgs:   defaultArgs,
		CommandLine: `geneos delete [TYPE] [NAME...]`,
		Summary:     `Delete an instance. Instance must be stopped.`,
		Description: `Delete the matching instances. This will only work on instances that are disabled to prevent
accidental deletion. The instance directory is removed without being backed-up. The user running
the command must have the appropriate permissions and a partial deletion cannot be protected
against.`}

	deleteFlags = flag.NewFlagSet("delete", flag.ExitOnError)
	deleteFlags.BoolVar(&deleteForced, "f", false, "Override need to have disabled instances")
	deleteFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	commands["rebuild"] = Command{
		Function:    commandRebuild,
		ParseFlags:  rebuildFlag,
		ParseArgs:   defaultArgs,
		CommandLine: `geneos rebuild [-f] [-n] [TYPE] {NAME...]`,
		Summary:     `Rebuild instance configuration files`,
		Description: `Rebuild instance configuration files based on current templates and instance configuration values
		
FLAGS:

	-f	Force rebuild for instances marked 'initial' even if configuration is not 'always' - 'never' is never rebuilt
	-n	No restart of instances`,
	}

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
	readConfigFile(LOCAL, globalConfig, &GlobalConfig)

	// root should not have a per-user config, but if sun by sudo the
	// HOME dir is conserved, so allow for now
	userConfDir, _ := os.UserConfigDir()
	err := readConfigFile(LOCAL, filepath.Join(userConfDir, "geneos.json"), &GlobalConfig)
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

func commandShow(ct Component, names []string, params []string) (err error) {
	// default to combined global + user config
	// allow overrides to show specific or components
	if len(names) == 0 {
		// special case "config show" for resolved settings
		printConfigStructJSON(GlobalConfig)
		return
	}

	// read the cofig into a struct then print it out again,
	// to sanitise the contents - or generate an error
	switch names[0] {
	case "global":
		var c GlobalSettings
		readConfigFile(LOCAL, globalConfig, &c)
		printConfigStructJSON(c)
		return
	case "user":
		var c GlobalSettings
		userConfDir, _ := os.UserConfigDir()
		readConfigFile(LOCAL, filepath.Join(userConfDir, "geneos.json"), &c)
		printConfigStructJSON(c)
		return
	}

	// loop instances - parse the args again and load/print the config,
	// but allow for RC files again
	var cs []Instances
	for _, name := range names {
		cs = append(cs, ct.instanceMatches(name)...)
	}
	printConfigSliceJSON(cs)

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

// components - parse the args again and load/print the config,
// but allow for RC files again
//
// consume component names, stop at first parameter, error out if more names
func commandSet(ct Component, args []string, params []string) (err error) {
	var instances []Instances

	logDebug.Println("args", args, "params", params)

	if len(args) == 0 && len(params) == 0 {
		return ErrInvalidArgs
	}

	if len(args) == 0 {
		// if all args have no become params (e.g. 'set gateway X=Y') then reprocess args here
		args = ct.instanceNames(ALL)
	} else {
		if args[0] == "user" {
			userConfDir, _ := os.UserConfigDir()
			return writeConfigParams(filepath.Join(userConfDir, "geneos.json"), params)
		}

		if args[0] == "global" {
			return writeConfigParams(globalConfig, params)
		}
	}

	// loop through named instances
	for _, arg := range args {
		instances = append(instances, ct.instanceMatches(arg)...)
	}

	for _, arg := range params {
		// special handling for "Env" field, which is always
		// a slice of environment key=value pairs
		// 'geneos set probe Env=JAVA_HOME=/path'
		// remove with leading '-' ?
		// 'geneos set probe Env=-PASSWORD'
		s := strings.SplitN(arg, "=", 2)
		if len(s) != 2 {
			logError.Printf("ignoring %q %s", arg, ErrInvalidArgs)
			continue
		}
		k, v := s[0], s[1]

		defaults := map[string]string{
			"Includes": "100",
			"Gateways": "7039",
		}

		// loop through all provided instances, set the parameter(s)
		for _, c := range instances {
			for _, vs := range strings.Split(v, ",") {
				switch k {
				// make this list dynamic
				case "Includes", "Gateways":
					var remove bool
					e := strings.SplitN(vs, ":", 2)
					if strings.HasPrefix(e[0], "-") {
						e[0] = strings.TrimPrefix(e[0], "-")
						remove = true
					}
					if remove {
						err = setStructMap(c, k, e[0], "")
						if err != nil {
							log.Fatalln(err)
						}
					} else {
						val := defaults[k]
						if len(e) > 1 {
							val = e[1]
						}
						err = setStructMap(c, k, e[0], val)
						if err != nil {
							log.Fatalln(err)
						}
					}
				case "Attributes":
					var remove bool
					e := strings.SplitN(vs, "=", 2)
					if strings.HasPrefix(e[0], "-") {
						e[0] = strings.TrimPrefix(e[0], "-")
						remove = true
					}
					// '-name' or 'name=' remove the attribute
					if remove || len(e) == 1 {
						err = setStructMap(c, k, e[0], "")
						if err != nil {
							log.Fatalln(err)
						}
					} else {
						err = setStructMap(c, k, e[0], e[1])
						if err != nil {
							log.Fatalln(err)
						}
					}
				case "Env", "Types":
					var remove bool
					slice := getSliceStrings(c, k)
					e := strings.SplitN(vs, "=", 2)
					if strings.HasPrefix(e[0], "-") {
						e[0] = strings.TrimPrefix(e[0], "-")
						remove = true
					}
					anchor := "="
					if remove && strings.HasSuffix(e[0], "*") {
						// wildcard removal (only)
						e[0] = strings.TrimSuffix(e[0], "*")
						anchor = ""
					}
					var exists bool
					// transfer ietms to new slice as removing items in a loop
					// does random things
					var newslice []string
					for _, n := range slice {
						if strings.HasPrefix(n, e[0]+anchor) {
							if !remove {
								// replace with new value
								newslice = append(newslice, vs)
								exists = true
							}
						} else {
							// copy existing
							newslice = append(newslice, n)
						}
					}
					// add a new item rather than update or remove
					if !exists && !remove {
						newslice = append(newslice, vs)
					}
					if err = setFieldSlice(c, k, newslice); err != nil {
						return
					}
				default:
					if err = setField(c, k, vs); err != nil {
						return
					}
				}
			}
		}
	}

	// now loop through the collected results and write out
	for _, c := range instances {
		if err = writeInstanceConfig(c); err != nil {
			log.Fatalln(err)
		}
	}

	return
}

func writeConfigParams(filename string, params []string) (err error) {
	var c GlobalSettings
	// ignore err - config may not exist, but that's OK
	_ = readConfigFile(LOCAL, filename, &c)
	// change here
	if len(c) == 0 {
		c = make(GlobalSettings)
	}
	for _, set := range params {
		// skip all non '=' args
		if !strings.Contains(set, "=") {
			continue
		}
		s := strings.SplitN(set, "=", 2)
		k, v := s[0], s[1]
		c[Global(k)] = v
	}

	// XXX fix permissions assumptions here
	if filename == globalConfig {
		return writeConfigFile(LOCAL, filename, "root", c)
	}
	return writeConfigFile(LOCAL, filename, "", c)
}

func writeInstanceConfig(c Instances) (err error) {
	err = writeConfigFile(c.Location(), filepath.Join(c.Home(), c.Type().String()+".json"), c.Prefix("User"), c)
	return
}

func readConfigFile(remote RemoteName, file string, config interface{}) (err error) {
	jsonFile, err := readFile(remote, file)
	if err != nil {
		return
	}
	// dec := json.NewDecoder(jsonFile)
	return json.Unmarshal(jsonFile, &config)
}

// try to be atomic, lots of edge cases, UNIX/Linux only
// we know the size of config structs is typicall small, so just marshal
// in memory
func writeConfigFile(remote RemoteName, file string, username string, config interface{}) (err error) {
	j, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return
	}

	uid, gid := -1, -1
	if superuser {
		if username == "" {
			logError.Panicln("cannot find non-root user to write config file", file)
		}
		ux, gx, _, err := getUser(username)
		if err != nil {
			return err
		}
		uid, gid = int(ux), int(gx)
	}

	dir := filepath.Dir(file)
	// try to ensure directory exists
	if err = mkdirAll(remote, dir, 0775); err != nil {
		return
	}
	// change final directory ownership
	_ = chown(remote, dir, uid, gid)

	buffer := bytes.NewBuffer(j)
	f, fn, err := createTempFile(remote, file, 0664)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = chown(remote, file, int(uid), int(gid)); err != nil {
		removeFile(remote, file)
	}

	if _, err = io.Copy(f, buffer); err != nil {
		return err
	}

	return renameFile(remote, fn, file)

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

	if err = renameFile(oldconf.Location(), oldhome, newhome); err != nil {
		logDebug.Println("rename failed:", oldhome, newhome, err)
		return
	}

	if err = setField(oldconf, "Name", newname); err != nil {
		// try to recover
		_ = renameFile(newconf.Location(), newhome, oldhome)
		return
	}
	if err = setField(oldconf, oldconf.Prefix("Home"), filepath.Join(ct.componentDir(newconf.Location()), newname)); err != nil {
		// try to recover
		_ = renameFile(newconf.Location(), newhome, oldhome)
		return
	}

	// config changes don't matter until writing config succeeds
	if err = writeConfigFile(newconf.Location(), filepath.Join(newhome, ct.String()+".json"), newconf.Prefix("User"), oldconf); err != nil {
		_ = renameFile(newconf.Location(), newhome, oldhome)
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
		if err = removeAll(c.Location(), c.Home()); err != nil {
			logError.Fatalln(err)
		}
		return nil
	}

	if Disabled(c) {
		if err = removeAll(c.Location(), c.Home()); err != nil {
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
	log.Printf("%s %s@%s configuration rebuilt", c.Type(), c.Name(), c.Location())
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
