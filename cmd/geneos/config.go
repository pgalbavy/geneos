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

func init() {
	commands["init"] = Command{initCommand, nil, `geneos init [username] [directory]`,
		`Initialise a geneos installation by creating a suitable directory hierarhcy and
	a user configuration file, setting the username and directory given as defaults.
	If 'username' is supplied then the command must either be run as root or the given user.
	If 'directory' is given then the parent directory must be writable by the user, unless
	running as root, otherwise a 'geneos' directory is created in the user's home area.
	If run as root a username MUST be given and only the username specific configuration
	file is created. If the directory exists then it must be empty. Exmaple:

		sudo geneos init geneos /opt/itrs
`}

	commands["migrate"] = Command{commandMigrate, parseArgs, "geneos migrate [TYPE] [NAME...]",
		`Migrate any legacy .rc configuration files to JSON format and rename the .rc file to
.rc.orig.`}
	commands["revert"] = Command{commandRevert, parseArgs, "geneos revert [TYPE] [NAME...]",
		`Revert migration of legacy .rc files to JSON ir the .rc.orig backup file still exists.
Any changes to the instance configuration since initial migration will be lost as the .rc file
is never written to.`}

	commands["show"] = Command{commandShow, parseArgs,
		`geneos show
	geneos show [global|user]
	geneos show [TYPE] [NAME...]`,
		`Show the JSON format configuration. With no arguments show the running configuration that
results from loading the global and user configurations and resolving any enviornment variables that
override scope. If the liternal keyword 'global' or 'user' is supplied then any on-disk configuration
for the respective options will be shown. If a component TYPE and/or instance NAME(s) are supplied
then the JSON configuration for those instances are output as a JSON array. This is regardless of the
instance using a legacy .rc file or a native JSON configuration.

Passwords and secrets are redacted in a very simplistic manner simply to prevent visibility in
casual viewing.`}

	commands["set"] = Command{commandSet, parseArgs,
		`geneos set [global|user] KEY=VALUE [KEY=VALUE...]
	geneos set [TYPE] [NAME...] KEY=VALUE [KEY=VALUE...]`,
		``}

	commands["disable"] = Command{commandDisable, parseArgs, "geneos disable [TYPE] [NAME...]",
		`Mark any matching instances as disabled. The instances are also stopped.`}

	commands["enable"] = Command{commandEneable, parseArgs, "geneos enable [TYPE] [NAME...]",
		`Mark any matcing instances as enabled and if this is a change then start the instance.`}

	commands["rename"] = Command{commandRename, nil, "geneos rename [TYPE] FROM TO",
		`Rename the matching instance. TYPE is optional to resolve any ambiguities if two instances
share the same name. No configuration changes outside the instance JSON config file are done. As
any existing .rc legacy file is never changed, this will migrate the instance from .rc to JSON.
The instance is stopped and restarted after the instance directory and configuration are changed.
It is an error to try to rename an instance to one that already exists with the same name.
	
NOT YET IMPLEMENED.`}

	commands["delete"] = Command{commandDelete, parseArgs, "geneos delete [TYPE] [NAME...]",
		`Delete the matching instances. This will only work on instances that are disabled to prevent
accidental deletion. The instance directory is removed without being backed-up. The user running
the command must have the appropriate permissions and a partial deletion cannot be protected
against.`}

}

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
func initCommand(ct ComponentType, args []string) (err error) {
	var c ConfigType
	if ct != None {
		log.Fatalln("cannot initialise with component type", ct, "given")
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
		log.Fatalln("init requires a user when run as root")
	}
	username := args[0]
	uid, gid, _, err := getUser(username)

	if err != nil {
		log.Fatalln("invalid user", username)
	}
	u, err := user.Lookup(username)
	if err != nil {
		log.Fatalln("user lookup failed")
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
			log.Fatalln(err)
		}
		if len(dirs) != 0 {
			log.Fatalln("directory exists and is not empty")
		}
	} else {
		// need to create out own, chown new directories only
		os.MkdirAll(dir, 0775)
	}
	if err = os.Chown(dir, uid, gid); err != nil {
		log.Fatalln(err)
	}
	c.ITRSHome = dir
	c.DefaultUser = username
	if err = writeConfigFile(globalConfig, c); err != nil {
		log.Fatalln("cannot write global config", err)
	}
	// if everything else worked, remove any existing user config
	_ = os.Remove(filepath.Join(u.HomeDir, ".config", "geneos.json"))

	// create directories
	for _, d := range initDirs {
		dir := filepath.Join(c.ITRSHome, d)
		os.MkdirAll(dir, 0775)
	}
	err = filepath.WalkDir(c.ITRSHome, func(path string, dir fs.DirEntry, err error) error {
		if err == nil {
			log.Println("chown", path, uid, gid)
			err = os.Chown(path, uid, gid)
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
		log.Fatalln("too many args")
	}

	// dir must first not exist (or be empty) and then be createable
	if _, err = os.Stat(dir); err == nil {
		// check empty
		dirs, err := os.ReadDir(dir)
		if err != nil {
			log.Fatalln(err)
		}
		if len(dirs) != 0 {
			log.Fatalf("target directory %q exists and is not empty", dir)
		}
	} else {
		// need to create out own, chown new directories only
		os.MkdirAll(dir, 0775)
	}

	userConfDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalln("no user config directory")
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
		os.MkdirAll(dir, 0775)
	}
	return
}

func commandMigrate(ct ComponentType, names []string) (err error) {
	return loopCommand(migrateInstance, ct, names)
}

func migrateInstance(c Instance) (err error) {
	if err = loadConfig(c, true); err != nil {
		log.Println(Type(c), Name(c), "cannot migrate configuration", err)
	}
	return
}

func commandRevert(ct ComponentType, names []string) (err error) {
	return loopCommand(revertInstance, ct, names)
}

func revertInstance(c Instance) (err error) {
	baseconf := filepath.Join(Home(c), Type(c).String())

	// if *.rc file exists, remove rc.orig+JSON, continue
	if _, err := os.Stat(baseconf + ".rc"); err == nil {
		// ignore errors
		if os.Remove(baseconf+".rc.orig") == nil || os.Remove(baseconf+".json") == nil {
			DebugLog.Println(Type(c), Name(c), "removed extra config file(s)")
		}
		return err
	}

	if err = os.Rename(baseconf+".rc.orig", baseconf+".rc"); err != nil {
		return
	}

	if err = os.Remove(baseconf + ".json"); err != nil {
		return
	}

	DebugLog.Println(Type(c), Name(c), "reverted to RC config")
	return nil
}

func commandShow(ct ComponentType, names []string) (err error) {
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

func printConfigSliceJSON(Slice []Instance) (err error) {
	s := "["
	for _, i := range Slice {
		if x, err := marshalStruct(i, "    "); err == nil {
			s += "\n    " + x + "\n"
		}
	}
	s += "]"
	log.Println(s)

	return

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

// set a (or multiple?) configuration parameters
//
// global or user update the respective config files
//
// when supplied a component type and name it applies
// to the JSON config for that component. A migration is performed if the
// current config is in RC format
//
// so we support "all" to do global updates of a parameter?
//
// format:
// geneos set gateway wonderland GatePort=8888
// geneos set global ITRSHome=/opt/geneos
//
// quoting is left to the shell rules and the setting is just split on the first '='
// non '=' args are taken as other names?
//
// What is read only? Name, others?
//
// support for Env slice in probe (and generally)
//
func commandSet(ct ComponentType, args []string) (err error) {
	if len(args) == 0 {
		return os.ErrInvalid
	}

	// read the cofig into a struct, make changes, then save it out again,
	// to sanitise the contents - or generate an error
	switch args[0] {
	case "global":
		return setConfig(globalConfig, args[1:])
	case "user":
		userConfDir, _ := os.UserConfigDir()
		return setConfig(filepath.Join(userConfDir, "geneos.json"), args[1:])
	}

	// check if all args have an '=' - if so default to "set user"
	eqs := len(args)
	for _, arg := range args {
		if strings.Contains(arg, "=") {
			eqs--
		}
	}
	if eqs == 0 {
		userConfDir, _ := os.UserConfigDir()
		setConfig(filepath.Join(userConfDir, "geneos.json"), args)
		return
	}

	// components - parse the args again and load/print the config,
	// but allow for RC files again
	//
	// consume component names, stop at first parameter, error out if more names
	var cs []Instance
	var setFlag bool

	for _, arg := range args {
		if !strings.Contains(arg, "=") {
			// if any settings have been seen but there is a non-setting
			// then stop processing, maybe error out
			if setFlag {
				log.Println("already found settings")
				// error out
				break
			}

			// this is still an instance name, squirrel away and loop
			for _, c := range NewComponent(ct, arg) {
				// migration required to set values
				if err = loadConfig(c, true); err != nil {
					log.Println(Type(c), Name(c), "cannot load configuration")
					continue
				}
				cs = append(cs, c)
			}
			continue
		}

		// special handling for "Env" field, which is always
		// a slice of environment key=value pairs
		// 'geneos set probe Env=JAVA_HOME=/path'
		// remove with leading '-' ?
		// 'geneos set probe Env=-PASSWORD'
		s := strings.SplitN(arg, "=", 2)
		k, v := s[0], s[1]

		// loop through all provided components, set the parameter(s)
		for _, c := range cs {
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
				setFieldSlice(c, k, newenv)
			} else {
				setField(c, k, v)
			}
			setFlag = true
		}
	}

	// now loop through the collected results anbd write out
	for _, c := range cs {
		conffile := filepath.Join(Home(c), Type(c).String()+".json")
		if err = writeConfigFile(conffile, c); err != nil {
			log.Println(err)
		}
	}

	return

}

// set envs?
// geneos set gateway test Env+X=Y Env-Z Env=A=B
//
func setConfig(filename string, args []string) (err error) {
	var c ConfigType
	// ignore err - config may not exist, but that's OK
	_ = readConfigFile(filename, &c)
	// change here
	for _, set := range args {
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
	if f, err := os.ReadFile(file); err == nil {
		return json.Unmarshal(f, &config)
	}
	return
}

// try to be atomic, lots of edge cases, UNIX/Linux only
func writeConfigFile(file string, config interface{}) (err error) {
	// marshal
	buffer, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return
	}

	// get these early
	uid := -1
	gid := -1
	if superuser {
		username := getString(config, Prefix(config)+"User")
		if username != "" {
			if uid, gid, _, err = getUser(username); err != nil {
				return err
			}
		}
	}

	// atomic-ish write
	dir, name := filepath.Split(file)
	// try to ensure directory exists
	if err = os.MkdirAll(dir, 0775); err == nil && superuser {
		// remember to change directory ownership
		os.Chown(dir, uid, gid)
	}
	f, err := os.CreateTemp(dir, name)
	if err != nil {
		err = fmt.Errorf("cannot create %q: %w", file, errors.Unwrap(err))
		return
	}
	defer os.Remove(f.Name())
	if _, err = fmt.Fprintln(f, string(buffer)); err != nil {
		return
	}

	if err = f.Chown(uid, gid); err != nil {
		return err
	}

	// XXX - these should not be hardwired
	if err = f.Chmod(0664); err != nil {
		return
	}
	return os.Rename(f.Name(), file)
}

const disableExtension = ".disabled"

// geneos disable gateway example ...
// stop if running first
// if run as root, the disable file is owned by root too and
// only root can remove it?
func commandDisable(ct ComponentType, args []string) (err error) {
	return loopCommand(disableInstance, ct, args)
}

func disableInstance(c Instance) (err error) {
	if isDisabled(c) {
		return fmt.Errorf("already disabled")
	}

	if err = stopInstance(c); err != nil && err != ErrProcNotExist {
		return err
	}
	d := filepath.Join(Home(c), Type(c).String()+disableExtension)
	f, err := os.Create(d)
	if err != nil {
		return err
	}
	defer f.Close()

	uid, gid, _, err := getUser(getString(c, Prefix(c)+"User"))
	if err != nil {
		os.Remove(f.Name())
		return
	}

	if err = f.Chown(uid, gid); err != nil {
		os.Remove(f.Name())
	}
	return
}

// simpler than disable, just try to remove the flag file
// we do also start the component(s)
func commandEneable(ct ComponentType, args []string) (err error) {
	return loopCommand(enableInstance, ct, args)
}

func enableInstance(c Instance) (err error) {
	d := filepath.Join(Home(c), Type(c).String()+disableExtension)
	if err = os.Remove(d); err == nil || errors.Is(err, os.ErrNotExist) {
		err = startInstance(c)
	}
	return
}

// an error tends to mean the disabled file is not
// visible, so assume it's not disabled - other things
// may be wrong but...
func isDisabled(c Instance) bool {
	d := filepath.Join(Home(c), Type(c).String()+disableExtension)
	if f, err := os.Stat(d); err == nil && f.Mode().IsRegular() {
		return true
	}
	return false
}

// this is special and the normal loopCommand will not work
// 'geneos rename gateway abc xyz'
//
// make sure old instance is down, new instance doesn't already exist
// clean up old instance
// rename directory
// migrate RC
// change config options the include name
// if gateway then rename in config?
func commandRename(ct ComponentType, args []string) (err error) {
	if len(args) != 2 {
		return ErrInvalidArgs
	}
	to := NewComponent(ct, args[1])[0]
	if err = loadConfig(to, false); err == nil {
		return fmt.Errorf("%s: %w", args[1], fs.ErrExist)
	}
	from := NewComponent(ct, args[0])[0]
	if err = loadConfig(from, true); err != nil {
		log.Println(Type(from), Name(from), "cannot load configuration")
	}
	pid, _ := findProc(from)
	if pid != 0 {
		return ErrProcExists
	}

	return ErrNotSupported
}

// also special - each component must be an exact match
// do we want a disable then delete protection?
func commandDelete(ct ComponentType, args []string) (err error) {
	return loopCommand(deleteInstance, ct, args)
}

func deleteInstance(c Instance) (err error) {
	if isDisabled(c) {
		if err = os.RemoveAll(Home(c)); err != nil {
			log.Fatalln(err)
		}
		return nil
	}
	log.Println(Type(c), Name(c), "must be disabled before delete")
	return nil
}
