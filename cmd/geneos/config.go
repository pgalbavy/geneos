package main

import (
	"encoding/json"
	"errors"
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
		ParseFlags:  nil,
		ParseArgs:   nil,
		CommandLine: `geneos init [username] [directory]`,
		Description: `Initialise a geneos installation by creating a suitable directory hierarhcy and
	a user configuration file, setting the username and directory given as defaults.
	If 'username' is supplied then the command must either be run as root or the given user.
	If 'directory' is given then the parent directory must be writable by the user, unless
	running as root, otherwise a 'geneos' directory is created in the user's home area.
	If run as root a username MUST be given and only the username specific configuration
	file is created. If the directory exists then it must be empty. Exmaple:

		sudo geneos init geneos /opt/itrs
`}

	commands["migrate"] = Command{
		Function:    commandMigrate,
		ParseFlags:  nil,
		ParseArgs:   parseArgs,
		CommandLine: "geneos migrate [TYPE] [NAME...]",
		Description: `Migrate any legacy .rc configuration files to JSON format and rename the .rc file to
.rc.orig.`}

	commands["revert"] = Command{
		Function:    commandRevert,
		ParseFlags:  nil,
		ParseArgs:   parseArgs,
		CommandLine: "geneos revert [TYPE] [NAME...]",
		Description: `Revert migration of legacy .rc files to JSON ir the .rc.orig backup file still exists.
Any changes to the instance configuration since initial migration will be lost as the .rc file
is never written to.`}

	commands["show"] = Command{
		Function:   commandShow,
		ParseFlags: nil,
		ParseArgs:  parseArgs,
		CommandLine: `geneos show
geneos show [global|user]
geneos show [TYPE] [NAME...]`,
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
		ParseFlags: nil,
		ParseArgs:  parseArgs,
		CommandLine: `geneos set [global|user] KEY=VALUE [KEY=VALUE...]
geneos set [TYPE] [NAME...] KEY=VALUE [KEY=VALUE...]`,
		Description: ``}

	commands["rename"] = Command{
		Function:    commandRename,
		ParseFlags:  nil,
		ParseArgs:   checkComponentArg,
		CommandLine: "geneos rename TYPE FROM TO",
		Description: `Rename the matching instance. TYPE is requied to resolve any ambiguities if two instances
share the same name. No configuration changes outside the instance JSON config file are done. As
any existing .rc legacy file is never changed, this will migrate the instance from .rc to JSON.
The instance is stopped and restarted after the instance directory and configuration are changed.
It is an error to try to rename an instance to one that already exists with the same name.`}

	commands["delete"] = Command{
		Function:    commandDelete,
		ParseFlags:  nil,
		ParseArgs:   parseArgs,
		CommandLine: "geneos delete [TYPE] [NAME...]",
		Description: `Delete the matching instances. This will only work on instances that are disabled to prevent
accidental deletion. The instance directory is removed without being backed-up. The user running
the command must have the appropriate permissions and a partial deletion cannot be protected
against.`}

}

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

	GatewayPortRange  string `json:",omitempty"`
	NetprobePortRange string `json:",omitempty"`
	LicdPortRange     string `json:",omitempty"`

	// Instance clean-up globs, two per type. Use PathListSep ':'
	GatewayCleanList  string `json:",omitempty"`
	GatewayPurgeList  string `json:",omitempty"`
	NetprobeCleanList string `json:",omitempty"`
	NetprobePurgeList string `json:",omitempty"`
	LicdCleanList     string `json:",omitempty"`
	LicdPurgeList     string `json:",omitempty"`
}

var RunningConfig ConfigType

var initDirs = []string{
	"packages/netprobe",
	"packages/gateway",
	"packages/licd",
	"netprobe/netprobes",
	"gateway/gateways",
	"gateway/gateway_shared",
	"gateway/gateway_config",
	"licd/licds",
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
	checkDefault(&RunningConfig.GatewayCleanList, defaultGatewayCleanList)
	checkDefault(&RunningConfig.GatewayPurgeList, defaultGatewayPurgeList)
	checkDefault(&RunningConfig.NetprobeCleanList, defaultNetprobeCleanList)
	checkDefault(&RunningConfig.NetprobePurgeList, defaultNetprobePurgeList)
	checkDefault(&RunningConfig.LicdCleanList, defaultLicdCleanList)
	checkDefault(&RunningConfig.LicdPurgeList, defaultLicdPurgeList)
}

func checkDefault(v *string, d string) {
	if *v == "" {
		*v = d
	}
}

//
// initialise a geneos installation
//
// take defaults from global or user config if they exist
// if not then parameters:
// defaults - current (non-root) user and home directory
// or
// e.g. 'sudo geneos init username /homedir'
// also creates config files if they don't exist, but no
// update to allow multiple parallel installs
//
// if not called as superuser "do the right thing", set
// ownerships to current user, create user config file
// and not global
//
// as root:
// 'geneos init user [dir]' = dir defaults to $HOME/geneos for the user given
// - global config *is* overwritten, user config is removed
//
// as user:
// 'geneos init [dir]' - similarly dir defaults to $HOME/geneos, providing user
// is an error. user config is overwritten
//
func commandInit(ct ComponentType, args []string, params []string) (err error) {
	var c ConfigType
	if ct != None {
		return ErrInvalidArgs
	}

	if superuser {
		err = initAsRoot(&c, args)
	} else {
		err = initAsUser(&c, args)
	}
	return
}

func initAsRoot(c *ConfigType, args []string) (err error) {
	if len(args) == 0 {
		logError.Fatalln("init requires a user when run as root")
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
		// if a user's homedir then add geneos, unless ?
		dir = filepath.Join(u.HomeDir, "geneos")
	} else {
		// must be an absolute path or relative to given user's home
		dir = args[1]
		if !strings.HasPrefix(dir, "/") {
			dir = filepath.Join(u.HomeDir, dir)
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
		dir = filepath.Join(u.HomeDir, "geneos")
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
		if len(dirs) != 0 {
			logError.Fatalf("target directory %q exists and is not empty", dir)
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
	if err = readConfigFile(filename, &c); err != nil {
		return
	}
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
	// try to ensure directory exists
	if err = os.MkdirAll(dir, 0775); err == nil {
		// change final directory ownership
		_ = os.Chown(dir, uid, gid)
	}
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
		return err
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
