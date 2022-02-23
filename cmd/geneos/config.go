package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

func init() {
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
		ParseArgs:   parseArgs,
		CommandLine: "geneos migrate [TYPE] [NAME...]",
		Summary:     `Migrate legacy .rc configuration to .json`,
		Description: `Migrate any legacy .rc configuration files to JSON format and rename the .rc file to
.rc.orig.`}

	commands["revert"] = Command{
		Function:    commandRevert,
		ParseFlags:  defaultFlag,
		ParseArgs:   parseArgs,
		CommandLine: `geneos revert [TYPE] [NAME...]`,
		Summary:     `Revert migration of .rc files from backups.`,
		Description: `Revert migration of legacy .rc files to JSON ir the .rc.orig backup file still exists.
Any changes to the instance configuration since initial migration will be lost as the .rc file
is never written to.`}

	commands["show"] = Command{
		Function:   commandShow,
		ParseFlags: defaultFlag,
		ParseArgs:  parseArgs,
		CommandLine: `geneos show
	geneos show [global|user]
	geneos show [TYPE] [NAME...]`,
		Summary: `Show runtime, global, user or instance configuration is JSON format`,
		Description: `Show the JSON format configuration. With no arguments show the running configuration that
results from loading the global and user configurations and resolving any enviornment variables that
override scope. If the liternal keyword 'global' or 'user' is supplied then any on-disk configuration
for the respective options will be shown. If a component TYPE and/or instance NAME(s) are supplied
then the JSON configuration for those instances are output as a JSON array. This is regardless of the
instance using a legacy .rc file or a native JSON configuration.

Passwords and secrets are redacted in a very simplistic manner simply to prevent visibility in
casual viewing.`}

	commands["set"] = Command{
		Function:   commandSet,
		ParseFlags: defaultFlag,
		ParseArgs:  parseArgs,
		CommandLine: `geneos set [global|user] KEY=VALUE [KEY=VALUE...]
	geneos set [TYPE] [NAME...] KEY=VALUE [KEY=VALUE...]`,
		Summary:     `Set runtime, global, user or instance configuration parameters`,
		Description: `Set a value in the configuration of either the user, globally or for a specific instance.`}

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
		ParseArgs:   parseArgs,
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

// Todo:
//  Port ranges for new components
//
type ConfigType struct {
	// Root directory for all operations
	ITRSHome string `json:",omitempty"`

	// Root URL for all downloads of software archives
	DownloadURL  string `json:",omitempty"`
	DownloadUser string `json:",omitempty"`
	DownloadPass string `json:",omitempty"`

	// Username to start components if not explicitly defined
	// and we are running with elevated privileges
	//
	// When running as a normal user this is unused and
	// we simply test a defined user against the running user
	//
	// default is owner of ITRSHome
	DefaultUser string `json:",omitempty"`

	// Path List sperated additions to the reserved names list, over and above
	// any words matched by parseComponentName()
	ReservedNames string `json:",omitempty"`

	GatewayPortRange   string `json:",omitempty"`
	NetprobePortRange  string `json:",omitempty"`
	LicdPortRange      string `json:",omitempty"`
	WebserverPortRange string `json:",omitempty"`

	// Instance clean-up globs, two per type. Use PathListSep ':'
	GatewayCleanList   string `json:",omitempty"`
	GatewayPurgeList   string `json:",omitempty"`
	NetprobeCleanList  string `json:",omitempty"`
	NetprobePurgeList  string `json:",omitempty"`
	LicdCleanList      string `json:",omitempty"`
	LicdPurgeList      string `json:",omitempty"`
	WebserverCleanList string `json:",omitempty"`
	WebserverPurgeList string `json:",omitempty"`
}

var RunningConfig ConfigType

var initDirs = []string{
	"packages/netprobe",
	"packages/gateway",
	"packages/licd",
	"packages/webserver",
	"netprobe/netprobes",
	"gateway/gateways",
	"gateway/gateway_shared",
	"gateway/gateway_config",
	"licd/licds",
	"webserver/webservers",
}

// load system config from global and user JSON files and process any
// environment variables we choose
func loadSysConfig() {
	readConfigFile(globalConfig, &RunningConfig)

	// root should not have a per-user config, but if sun by sudo the
	// HOME dir is conserved, so allow for now
	userConfDir, _ := os.UserConfigDir()
	err := readConfigFile(filepath.Join(userConfDir, "geneos.json"), &RunningConfig)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Println(err)
	}

	// setting the environment variable - to match legacy programs - overrides
	// all others
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		RunningConfig.ITRSHome = h
	}

	// defaults - make this long chain simpler
	checkDefault(&RunningConfig.GatewayPortRange, gatewayPortRange)
	checkDefault(&RunningConfig.NetprobePortRange, netprobePortRange)
	checkDefault(&RunningConfig.LicdPortRange, licdPortRange)
	checkDefault(&RunningConfig.WebserverPortRange, webserverPortRange)
	checkDefault(&RunningConfig.GatewayCleanList, defaultGatewayCleanList)
	checkDefault(&RunningConfig.GatewayPurgeList, defaultGatewayPurgeList)
	checkDefault(&RunningConfig.NetprobeCleanList, defaultNetprobeCleanList)
	checkDefault(&RunningConfig.NetprobePurgeList, defaultNetprobePurgeList)
	checkDefault(&RunningConfig.LicdCleanList, defaultLicdCleanList)
	checkDefault(&RunningConfig.LicdPurgeList, defaultLicdPurgeList)
	checkDefault(&RunningConfig.WebserverCleanList, defaultWebserverCleanList)
	checkDefault(&RunningConfig.WebserverPurgeList, defaultWebserverPurgeList)
}

func checkDefault(v *string, d string) {
	if *v == "" {
		*v = d
	}
}

//
// initialise a geneos installation
//
// if no directory given and not running as root and the last component of the user's
// home direcvtory is NOT "geneos" then create a directory "geneos", else
//
//
func commandInit(ct ComponentType, args []string, params []string) (err error) {
	var c ConfigType

	// none of the arguments can be a reserved type
	if ct != None {
		return ErrInvalidArgs
	}

	// cannot pass both flags
	if initDemo && initAll != "" {
		return ErrInvalidArgs
	}

	if superuser {
		err = initAsRoot(&c, args)
	} else {
		err = initAsUser(&c, args)
	}

	// now reload config, after init
	loadSysConfig()

	// create a demo environment
	if initDemo {
		e := []string{}
		g := []string{"Demo Gateway"}
		n := []string{"localhost"}
		commandDownload(None, e, e)
		commandNew(Gateway, g, e)
		commandSet(Gateway, g, []string{"GateOpts=-demo"})
		commandNew(Netprobe, n, e)
		commandNew(Webserver, []string{"demo"}, e)
		// call parseArgs() on an empty list to populate for loopCommand()
		ct, args, params := parseArgs(e)
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
		commandNew(Licd, g, e)
		commandUpload(Licd, g, []string{"geneos.lic=" + initAll})
		commandNew(Gateway, g, e)
		commandNew(Netprobe, n, e)
		commandNew(Webserver, g, e)
		// call parseArgs() on an empty list to populate for loopCommand()
		ct, args, params := parseArgs(e)
		commandStart(ct, args, params)
		commandPS(ct, args, params)
		return nil
	}

	return
}

func initAsRoot(c *ConfigType, args []string) (err error) {
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
	if _, err := os.Stat(dir); err == nil {
		// check empty
		dirs, err := os.ReadDir(dir)
		if err != nil {
			logError.Fatalln(err)
		}
		if len(dirs) != 0 {
			logError.Fatalln("directory exists and is not empty")
		}
	} else {
		// need to create out own, chown base directory only
		if err = os.MkdirAll(dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}
	if err = os.Chown(dir, int(uid), int(gid)); err != nil {
		logError.Fatalln(err)
	}
	c.ITRSHome = dir
	c.DefaultUser = username
	if err = writeConfigFile(globalConfig, c); err != nil {
		logError.Fatalln("cannot write global config", err)
	}
	// if everything else worked, remove any existing user config
	_ = os.Remove(filepath.Join(u.HomeDir, ".config", "geneos.json"))

	// create directories
	for _, d := range initDirs {
		dir := filepath.Join(c.ITRSHome, d)
		if err = os.MkdirAll(dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}
	err = filepath.WalkDir(c.ITRSHome, func(path string, dir fs.DirEntry, err error) error {
		if err == nil {
			logDebug.Println("chown", path, uid, gid)
			err = os.Chown(path, int(uid), int(gid))
		}
		return err
	})
	return
}

func initAsUser(c *ConfigType, args []string) (err error) {
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
	if _, err = os.Stat(dir); err == nil {
		// check empty
		dirs, err := os.ReadDir(dir)
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
		if err = os.MkdirAll(dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}

	userConfDir, err := os.UserConfigDir()
	if err != nil {
		logError.Fatalln("no user config directory")
	}
	userConfFile := filepath.Join(userConfDir, "geneos.json")
	c.ITRSHome = dir
	c.DefaultUser = u.Username
	if err = writeConfigFile(userConfFile, c); err != nil {
		return
	}
	// create directories
	for _, d := range initDirs {
		dir := filepath.Join(c.ITRSHome, d)
		if err = os.MkdirAll(dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}
	return
}

func commandMigrate(ct ComponentType, names []string, params []string) (err error) {
	return loopCommand(migrateInstance, ct, names, params)
}

func migrateInstance(c Instance, params []string) (err error) {
	if err = loadConfig(c, true); err != nil {
		log.Println(Type(c), Name(c), "cannot migrate configuration", err)
	}
	return
}

func commandRevert(ct ComponentType, names []string, params []string) (err error) {
	return loopCommand(revertInstance, ct, names, params)
}

func revertInstance(c Instance, params []string) (err error) {
	baseconf := filepath.Join(Home(c), Type(c).String())

	// if *.rc file exists, remove rc.orig+JSON, continue
	if _, err := os.Stat(baseconf + ".rc"); err == nil {
		// ignore errors
		if os.Remove(baseconf+".rc.orig") == nil || os.Remove(baseconf+".json") == nil {
			logDebug.Println(Type(c), Name(c), "removed extra config file(s)")
		}
		return err
	}

	if err = os.Rename(baseconf+".rc.orig", baseconf+".rc"); err != nil {
		return
	}

	if err = os.Remove(baseconf + ".json"); err != nil {
		return
	}

	logDebug.Println(Type(c), Name(c), "reverted to RC config")
	return nil
}

func commandShow(ct ComponentType, names []string, params []string) (err error) {
	// default to combined global + user config
	// allow overrides to show specific or components
	if len(names) == 0 {
		// special case "config show" for resolved settings
		printConfigStructJSON(RunningConfig)
		return
	}

	// read the cofig into a struct then print it out again,
	// to sanitise the contents - or generate an error
	switch names[0] {
	case "global":
		var c ConfigType
		readConfigFile(globalConfig, &c)
		printConfigStructJSON(c)
		return
	case "user":
		var c ConfigType
		userConfDir, _ := os.UserConfigDir()
		readConfigFile(filepath.Join(userConfDir, "geneos.json"), &c)
		printConfigStructJSON(c)
		return
	}

	// loop instances - parse the args again and load/print the config,
	// but allow for RC files again
	var cs []Instance
	for _, name := range names {
		for _, c := range NewComponent(ct, name) {
			if err = loadConfig(c, false); err != nil {
				log.Println(Type(c), Name(c), "cannot load configuration")
				continue
			}
			if c != nil {
				cs = append(cs, c)
			}
		}
	}
	printConfigSliceJSON(cs)

	return
}

func printConfigSliceJSON(Slice []Instance) {
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

func commandSet(ct ComponentType, args []string, params []string) (err error) {
	logDebug.Println("args", args, "params", params)
	if len(args) == 0 && len(params) == 0 {
		return os.ErrInvalid
	}

	if len(args) == 0 {
		userConfDir, _ := os.UserConfigDir()
		setConfig(filepath.Join(userConfDir, "geneos.json"), params)
		return
	}

	// read the cofig into a struct, make changes, then save it out again,
	// to sanitise the contents - or generate an error
	switch args[0] {
	case "global":
		return setConfig(globalConfig, params)
	case "user":
		userConfDir, _ := os.UserConfigDir()
		return setConfig(filepath.Join(userConfDir, "geneos.json"), params)
	}

	// components - parse the args again and load/print the config,
	// but allow for RC files again
	//
	// consume component names, stop at first parameter, error out if more names
	var instances []Instance

	// loop through named instances
	for _, arg := range args {
		for _, c := range NewComponent(ct, arg) {
			// migration required to set values
			if err = loadConfig(c, true); err != nil {
				log.Println(Type(c), Name(c), "cannot load configuration")
				continue
			}
			instances = append(instances, c)
		}
		continue
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
			if k == "Env" {
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
			} else {
				if err = setField(c, k, v); err != nil {
					return
				}
			}
		}
	}

	// now loop through the collected results anbd write out
	for _, c := range instances {
		conffile := filepath.Join(Home(c), Type(c).String()+".json")
		if err = writeConfigFile(conffile, c); err != nil {
			log.Println(err)
		}
	}

	return
}

func setConfig(filename string, params []string) (err error) {
	var c ConfigType
	// ignore err - config may not exist, but that's OK
	_ = readConfigFile(filename, &c)
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
	return writeConfigFile(filename, c)
}

func readConfigFile(file string, config interface{}) (err error) {
	jsonFile, err := os.Open(file)
	if err != nil {
		return
	}
	dec := json.NewDecoder(jsonFile)
	return dec.Decode(&config)
}

// try to be atomic, lots of edge cases, UNIX/Linux only
// we know the size of config structs is typicall small, so just marshal
// in memory
func writeConfigFile(file string, config interface{}) (err error) {
	buffer, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return
	}

	uid, gid := -1, -1
	if superuser {
		username := getString(config, Prefix(config)+"User")
		if username == "" {
			logError.Fatalln("cannot find non-root user to write config file", file)
		}
		ux, gx, _, err := getUser(username)
		if err != nil {
			return err
		}
		uid, gid = int(ux), int(gx)
	}

	dir, name := filepath.Split(file)
	dir = strings.TrimSuffix(dir, "/")
	// try to ensure directory exists
	if err = os.MkdirAll(dir, 0775); err != nil {
		return
	}
	// change final directory ownership
	_ = os.Chown(dir, uid, gid)

	f, err := os.CreateTemp(dir, name)
	if err != nil {
		return fmt.Errorf("cannot create %q: %w", file, errors.Unwrap(err))
	}
	defer os.Remove(f.Name())
	// use Println to get a final newline
	if _, err = fmt.Fprintln(f, string(buffer)); err != nil {
		return
	}

	// update file perms and owner before final rename to overwrite
	// existing file
	if err = f.Chmod(0664); err != nil {
		return
	}
	if err = f.Chown(uid, gid); err != nil {
		return
	}

	return os.Rename(f.Name(), file)
}

func commandRename(ct ComponentType, args []string, params []string) (err error) {
	if ct == None || len(args) != 2 {
		return ErrInvalidArgs
	}

	oldname := args[0]
	newname := args[1]

	logDebug.Println("rename", ct, oldname, newname)
	oldconf := NewComponent(ct, oldname)[0]
	if err = loadConfig(oldconf, true); err != nil {
		return fmt.Errorf("%s %s not found", ct, oldname)
	}
	tos := NewComponent(ct, newname)
	newconf := tos[0]
	if len(tos) == 0 {
		return fmt.Errorf("%s %s: %w", ct, newname, ErrInvalidArgs)
	}
	if err = loadConfig(newconf, false); err == nil {
		return fmt.Errorf("%s %s already exists", ct, newname)
	}

	stopInstance(oldconf, nil)

	// save for recover, as config gets changed
	oldhome := Home(oldconf)
	newhome := Home(newconf)

	if err = os.Rename(oldhome, newhome); err != nil {
		logDebug.Println("rename failed:", oldhome, newhome, err)
		return
	}

	if err = setField(oldconf, "Name", newname); err != nil {
		// try to recover
		_ = os.Rename(newhome, oldhome)
		return
	}
	if err = setField(oldconf, Prefix(oldconf)+"Home", filepath.Join(componentDir(ct), newname)); err != nil {
		// try to recover
		_ = os.Rename(newhome, oldhome)
		return
		//
	}

	// config changes don't matter until writing config succeeds
	if err = writeConfigFile(filepath.Join(newhome, ct.String()+".json"), oldconf); err != nil {
		_ = os.Rename(newhome, oldhome)
		return
	}
	log.Println(ct, oldname, "renamed to", newname)
	return startInstance(oldconf, nil)
}

func commandDelete(ct ComponentType, args []string, params []string) (err error) {
	return loopCommand(deleteInstance, ct, args, params)
}

func deleteInstance(c Instance, params []string) (err error) {
	if isDisabled(c) {
		if err = os.RemoveAll(Home(c)); err != nil {
			logError.Fatalln(err)
		}
		return nil
	}
	log.Println(Type(c), Name(c), "must be disabled before delete")
	return nil
}

func initFlag(command string, args []string) []string {
	initFlags.Parse(args)
	checkHelpFlag(command)
	return initFlags.Args()
}
