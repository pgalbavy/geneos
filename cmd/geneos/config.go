package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

func init() {
	GlobalConfig = make(GlobalSettings)

	commands["init"] = Command{
		Function:    commandInit,
		ParseFlags:  initFlag,
		ParseArgs:   nil,
		CommandLine: `geneos init [-d] [-a FILE] [USERNAME] [DIRECTORY]`,
		Summary:     `Initialise a Geneos installation`,
		Description: `Initialise a Geneos installation by creating the directory hierarchy and
user configuration file, with the USERNAME and DIRECTORY if supplied.
DIRECTORY must be an absolute path and this is used to distinguish it
from USERNAME.

DIRECTORY defaults to ${HOME}/geneos for the selected user
unless the last compoonent of ${HOME} is 'geneos' in which case the
home directory is used. e.g. if the user is 'geneos' and the home
directory is '/opt/geneos' then that is used, but if it were a user
'itrs' which a home directory of '/home/itrs' then the directory
'home/itrs/geneos' would be used. This only applies when no DIRECTORY
is explicitly supplied.

When DIRECTORY is given it must be an absolute path and the parent
directory must be writable by the user - either running the command
or given as USERNAME.

DIRECTORY, whether explict or implied, must not exist or be empty of
all except "dot" files and directories.

When run with superuser privileges a USERNAME must be supplied and
only the configuration file for that user is created. e.g.:

	sudo geneos init geneos /opt/itrs

When USERNAME is supplied then the command must either be run with
superuser privileges or be run by the same user.

Flags:

If the "-d" flag is given then the command performs all the steps
necessary to initialise and start a basic system using the demo
features of the gateway to avoid need for a license file.

If the "-a" flag is given along with the path to a license file then
all the necessary steps are run to initialise a basic system using
simple names for all components.

The '-d' and '-a' flags are mutually exclusive.
`}

	initFlags = flag.NewFlagSet("init", flag.ExitOnError)
	initFlags.BoolVar(&initDemo, "d", false, "Perform initialisation steps for a demo setup and start environment")
	initFlags.StringVar(&initAll, "a", "", "Perform initialisation steps using provided license file and start environment")
	initFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	commands["migrate"] = Command{
		Function:    commandMigrate,
		ParseFlags:  defaultFlag,
		ParseArgs:   defaultArgs,
		CommandLine: "geneos migrate [TYPE] [NAME...]",
		Summary:     `Migrate legacy .rc configuration to .json`,
		Description: `Migrate any legacy .rc configuration files to JSON format and rename the .rc file to
.rc.orig.`}

	commands["revert"] = Command{
		Function:    commandRevert,
		ParseFlags:  defaultFlag,
		ParseArgs:   defaultArgs,
		CommandLine: `geneos revert [TYPE] [NAME...]`,
		Summary:     `Revert migration of .rc files from backups.`,
		Description: `Revert migration of legacy .rc files to JSON ir the .rc.orig backup file still exists.
Any changes to the instance configuration since initial migration will be lost as the .rc file
is never written to.`}

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
		ParseFlags:  defaultFlag,
		ParseArgs:   defaultArgs,
		CommandLine: `geneos delete [TYPE] [NAME...]`,
		Summary:     `Delete an instance. Instance must be stopped.`,
		Description: `Delete the matching instances. This will only work on instances that are disabled to prevent
accidental deletion. The instance directory is removed without being backed-up. The user running
the command must have the appropriate permissions and a partial deletion cannot be protected
against.`}

}

var initFlags *flag.FlagSet
var initDemo bool
var initAll string

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

//
// initialise a geneos installation
//
// if no directory given and not running as root and the last component of the user's
// home direcvtory is NOT "geneos" then create a directory "geneos", else
//
//
func commandInit(ct Component, args []string, params []string) (err error) {
	// none of the arguments can be a reserved type
	if ct != None {
		return ErrInvalidArgs
	}

	// cannot pass both flags
	if initDemo && initAll != "" {
		return ErrInvalidArgs
	}

	if superuser {
		err = initAsRoot(args)
	} else {
		err = initAsUser(args)
	}

	// now reload config, after init
	loadSysConfig()

	// create a demo environment
	if initDemo {
		e := []string{}
		g := []string{"Demo Gateway"}
		n := []string{"localhost"}
		commandDownload(None, e, e)
		commandAdd(Gateway, g, e)
		commandSet(Gateway, g, []string{"GateOpts=-demo"})
		commandAdd(Netprobe, n, e)
		commandAdd(Webserver, []string{"demo"}, e)
		// call defaultArgs() on an empty list to populate for loopCommand()
		ct, args, params := defaultArgs(e)
		commandStart(ct, args, params)
		commandPS(ct, args, params)
		return
	}

	// create a basic environment with license file
	if initAll != "" {
		h, err := os.Hostname()
		if err != nil {
			return err
		}
		e := []string{}
		g := []string{h}
		n := []string{"localhost"}
		commandDownload(None, e, e)
		commandAdd(Licd, g, e)
		commandImport(Licd, g, []string{"geneos.lic=" + initAll})
		commandAdd(Gateway, g, e)
		commandAdd(Netprobe, n, e)
		commandAdd(Webserver, g, e)
		// call defaultArgs() on an empty list to populate for loopCommand()
		ct, args, params := defaultArgs(e)
		commandStart(ct, args, params)
		commandPS(ct, args, params)
		return nil
	}

	return
}

func initAsRoot(args []string) (err error) {
	c := make(GlobalSettings)
	if len(args) == 0 {
		logError.Fatalln("init requires a USERNAME when run as root")
	}
	username := args[0]
	uid, gid, _, err := getUser(username)

	if err != nil {
		logError.Fatalln("invalid user", username)
	}
	u, err := user.Lookup(username)
	if err != nil {
		logError.Fatalln("user lookup failed")
	}

	var dir string
	if len(args) == 1 {
		// If user's home dir doesn't end in "geneos" then create a
		// directory "geneos" else use the home directory directly
		dir = u.HomeDir
		if filepath.Base(u.HomeDir) != "geneos" {
			dir = filepath.Join(u.HomeDir, "geneos")
		}
	} else {
		// must be an absolute path or relative to given user's home
		dir = args[1]
		if !strings.HasPrefix(dir, "/") {
			dir = u.HomeDir
			if filepath.Base(u.HomeDir) != "geneos" {
				dir = filepath.Join(u.HomeDir, dir)
			}
		}
	}

	// dir must first not exist (or be empty) and then be createable
	if _, err := statFile(LOCAL, dir); err == nil {
		// check empty
		dirs, err := readDir(LOCAL, dir)
		if err != nil {
			logError.Fatalln(err)
		}
		if len(dirs) != 0 {
			logError.Fatalln("directory exists and is not empty")
		}
	} else {
		// need to create out own, chown base directory only
		if err = mkdirAll(LOCAL, dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}
	if err = chown(LOCAL, dir, int(uid), int(gid)); err != nil {
		logError.Fatalln(err)
	}
	c["ITRSHome"] = dir
	c["DefaultUser"] = username
	if err = writeConfigFile(LOCAL, globalConfig, c); err != nil {
		logError.Fatalln("cannot write global config", err)
	}
	// if everything else worked, remove any existing user config
	_ = removeFile(LOCAL, filepath.Join(u.HomeDir, ".config", "geneos.json"))

	// create directories
	for _, d := range initDirs {
		dir := filepath.Join(c["ITRSHome"], d)
		if err = mkdirAll(LOCAL, dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}
	err = filepath.WalkDir(c["ITRSHome"], func(path string, dir fs.DirEntry, err error) error {
		if err == nil {
			err = chown(LOCAL, path, int(uid), int(gid))
		}
		return err
	})
	return
}

func initAsUser(args []string) (err error) {
	c := make(GlobalSettings)
	// normal user
	var dir string
	u, _ := user.Current()
	switch len(args) {
	case 0: // default home + geneos
		dir = u.HomeDir
		if filepath.Base(u.HomeDir) != "geneos" {
			dir = filepath.Join(u.HomeDir, "geneos")
		}
	case 1: // home = abs path
		dir, _ = filepath.Abs(args[0])
	default:
		logError.Fatalln("too many args")
	}

	// dir must first not exist (or be empty) and then be createable
	if _, err = statFile(LOCAL, dir); err == nil {
		// check empty
		dirs, err := readDir(LOCAL, dir)
		if err != nil {
			logError.Fatalln(err)
		}
		// ignore dot files
		for _, entry := range dirs {
			if !strings.HasPrefix(entry.Name(), ".") {
				logError.Fatalf("target directory %q exists and is not empty", dir)
			}
		}
	} else {
		// need to create out own, chown base directory only
		if err = mkdirAll(LOCAL, dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}

	userConfDir, err := os.UserConfigDir()
	if err != nil {
		logError.Fatalln("no user config directory")
	}
	userConfFile := filepath.Join(userConfDir, "geneos.json")
	c["ITRSHome"] = dir
	c["DefaultUser"] = u.Username
	if err = writeConfigFile(LOCAL, userConfFile, c); err != nil {
		return
	}
	// create directories
	for _, d := range initDirs {
		dir := filepath.Join(c["ITRSHome"], d)
		if err = mkdirAll(LOCAL, dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}
	return
}

func commandMigrate(ct Component, names []string, params []string) (err error) {
	return ct.loopCommand(migrateInstance, names, params)
}

func migrateInstance(c Instances, params []string) (err error) {
	if err = loadConfig(c, true); err != nil {
		log.Println(c.Type(), c.Name(), "cannot migrate configuration", err)
	}
	return
}

func commandRevert(ct Component, names []string, params []string) (err error) {
	return ct.loopCommand(revertInstance, names, params)
}

func revertInstance(c Instances, params []string) (err error) {
	baseconf := filepath.Join(c.Home(), c.Type().String())

	// if *.rc file exists, remove rc.orig+JSON, continue
	if _, err := statFile(c.Location(), baseconf+".rc"); err == nil {
		// ignore errors
		if removeFile(c.Location(), baseconf+".rc.orig") == nil || removeFile(c.Location(), baseconf+".json") == nil {
			logDebug.Println(c.Type(), c.Name(), "removed extra config file(s)")
		}
		return err
	}

	if err = renameFile(c.Location(), baseconf+".rc.orig", baseconf+".rc"); err != nil {
		return
	}

	if err = removeFile(c.Location(), baseconf+".json"); err != nil {
		return
	}

	logDebug.Println(c.Type(), c.Name(), "reverted to RC config")
	return nil
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

func commandSet(ct Component, args []string, params []string) (err error) {
	logDebug.Println("args", args, "params", params)
	if len(args) == 0 && len(params) == 0 {
		return os.ErrInvalid
	}

	if len(args) == 0 {
		userConfDir, _ := os.UserConfigDir()
		writeConfigParams(filepath.Join(userConfDir, "geneos.json"), params)
		return
	}

	// read the cofig into a struct, make changes, then save it out again,
	// to sanitise the contents - or generate an error
	switch args[0] {
	case "global":
		return writeConfigParams(globalConfig, params)
	case "user":
		userConfDir, _ := os.UserConfigDir()
		return writeConfigParams(filepath.Join(userConfDir, "geneos.json"), params)
	}

	// components - parse the args again and load/print the config,
	// but allow for RC files again
	//
	// consume component names, stop at first parameter, error out if more names
	var instances []Instances

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
		k, v := s[0], s[1]

		// loop through all provided instances, set the parameter(s)
		for _, c := range instances {
			switch k {
			case "Env", "Attributes", "Gateways", "Variables":
				var remove bool
				env := getSliceStrings(c, k)
				e := strings.SplitN(v, "=", 2)
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
				var newenv []string
				for _, n := range env {
					if strings.HasPrefix(n, e[0]+anchor) {
						if !remove {
							// replace with new value
							newenv = append(newenv, v)
							exists = true
						}
					} else {
						// copy existing
						newenv = append(newenv, n)
					}
				}
				// add a new item rather than update or remove
				if !exists && !remove {
					newenv = append(newenv, v)
				}
				if err = setFieldSlice(c, k, newenv); err != nil {
					return
				}
			default:
				if err = setField(c, k, v); err != nil {
					return
				}
			}
		}
	}

	// now loop through the collected results anbd write out
	for _, c := range instances {
		conffile := filepath.Join(c.Home(), c.Type().String()+".json")
		if err = writeConfigFile(c.Location(), conffile, c); err != nil {
			log.Println(err)
		}
	}

	return
}

func writeConfigParams(filename string, params []string) (err error) {
	var c GlobalSettings
	// ignore err - config may not exist, but that's OK
	_ = readConfigFile(LOCAL, filename, &c)
	// change here
	for _, set := range params {
		// skip all non '=' args
		if !strings.Contains(set, "=") {
			continue
		}
		s := strings.SplitN(set, "=", 2)
		k, v := s[0], s[1]
		if err = setField(&c, k, v); err != nil {
			return
		}
	}
	return writeConfigFile(LOCAL, filename, c)
}

func writeInstanceConfig(c Instances) (err error) {
	err = writeConfigFile(c.Location(), filepath.Join(c.Home(), c.Type().String()+".json"), c)
	return
}

func readConfigFile(remote, file string, config interface{}) (err error) {
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
func writeConfigFile(remote, file string, config interface{}) (err error) {
	j, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return
	}

	uid, gid := -1, -1
	if superuser {
		username := "" // getString(config, Prefix(config)+"User")
		if username == "" {
			logError.Fatalln("cannot find non-root user to write config file", file)
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
	oldconf := ct.New(oldname)
	if err = loadConfig(oldconf, true); err != nil {
		return fmt.Errorf("%s %s not found", ct, oldname)
	}
	newconf := ct.New(newname)
	if err = loadConfig(newconf, false); err == nil {
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
	if err = writeConfigFile(newconf.Location(), filepath.Join(newhome, ct.String()+".json"), oldconf); err != nil {
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
	if isDisabled(c) {
		if err = removeAll(c.Location(), c.Home()); err != nil {
			logError.Fatalln(err)
		}
		return nil
	}
	log.Println(c.Type(), c.Name(), "must be disabled before delete")
	return nil
}

func initFlag(command string, args []string) []string {
	initFlags.Parse(args)
	checkHelpFlag(command)
	return initFlags.Args()
}
