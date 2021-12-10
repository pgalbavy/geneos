package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
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
}

var Config ConfigType

func init() {
	commands["init"] = Command{initCommand, nil, "initialise"}
	commands["migrate"] = Command{migrateCommand, parseArgs, "migrate"}
	commands["revert"] = Command{revertCommand, parseArgs, "revert"}
	commands["show"] = Command{showCommand, parseArgs, "show"}
	commands["set"] = Command{setCommand, parseArgs, "set"}

	commands["disable"] = Command{disableCommand, parseArgs, "disable"}
	commands["enable"] = Command{enableCommand, parseArgs, "enable"}
	commands["rename"] = Command{renameCommand, parseArgs, "rename"}
	commands["delete"] = Command{deleteCommand, parseArgs, "delete"}
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
	readConfigFile(globalConfig, &Config)

	// root should not have a per-user config, but if sun by sudo the
	// HOME dir is conserved, so allow for now
	userConfDir, _ := os.UserConfigDir()
	err := readConfigFile(filepath.Join(userConfDir, "geneos.json"), &Config)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Println(err)
	}

	// setting the environment variable - to match legacy programs - overrides
	// all others
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		Config.ITRSHome = h
	}

	if Config.GatewayPortRange == "" {
		Config.GatewayPortRange = gatewayPortRange

	}

	if Config.NetprobePortRange == "" {
		Config.NetprobePortRange = netprobePortRange

	}

	if Config.LicdPortRange == "" {
		Config.LicdPortRange = licdPortRange
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
			log.Fatalln("directory exists and is not empty")
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

//
// there are two types of config, subdivided into further categories:
//
// 1. global and user general configs, including root dirs etc.
// 2. per-component configs
//
// the config command introduces "global" and "user" keywords so these need
// to be added to reserved lists too
//
// all "set" commands must only update the on-disk config of the selected
// config, and not write out a merged config loaded from layers of scoping
// resolution. all writes must also be as atomic as possible and not leave
// empty files or delete original files until the new one is ready.
//
// "migrate" is (only) for component configs and converts an RC file to a
// JSON, renames the old file. Can do multiple components at once. Should we
// have a "revert" command?
//

func migrateCommand(ct ComponentType, names []string) (err error) {
	if len(names) == 0 {
		return os.ErrInvalid
	}

	// do components - parse the args again and load/print the config,
	// but allow for RC files again
	for _, name := range names {
		for _, c := range New(ct, name) {
			// passing true here migrates the RC file, doing nothing ir already
			// in JSON format
			if err = loadConfig(c, true); err != nil {
				log.Println(Type(c), Name(c), "cannot migrate configuration", err)
				continue
			}
		}
	}

	return
}

// rename rc.orig to rc, remove JSON, return
//
func revertCommand(ct ComponentType, names []string) (err error) {
	if len(names) == 0 {
		return os.ErrInvalid
	}

	// do compoents - parse the args again and load/print the config,
	// but allow for RC files again
	for _, name := range names {
		for _, c := range New(ct, name) {
			// load a config, following normal logic, first
			if err = loadConfig(c, false); err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				continue
			}
			baseconf := filepath.Join(Home(c), Type(c).String())

			// if *.rc file exists, remove rc.orig+JSON, continue
			if _, err := os.Stat(baseconf + ".rc"); err == nil {
				// ignore errors
				if os.Remove(baseconf+".rc.orig") == nil || os.Remove(baseconf+".json") == nil {
					log.Println(Type(c), Name(c), "removed extra config file(s)")
				}
				continue
			}

			if err = os.Rename(baseconf+".rc.orig", baseconf+".rc"); err != nil {
				log.Println(Type(c), Name(c), err)
				continue
			}

			if err = os.Remove(baseconf + ".json"); err != nil {
				log.Println(Type(c), Name(c), err)
				continue
			}

			log.Println(Type(c), Name(c), "reverted to RC config")
		}
	}

	return
}

func showCommand(ct ComponentType, names []string) (err error) {
	// default to combined global + user config
	// allow overrides to show specific or components
	if len(names) == 0 {
		// special case "config show" for resolved settings
		printConfigJSON(Config)
		return
	}

	// read the cofig into a struct then print it out again,
	// to sanitise the contents - or generate an error
	switch names[0] {
	case "global":
		var c ConfigType
		readConfigFile(globalConfig, &c)
		printConfigJSON(c)
		return
	case "user":
		var c ConfigType
		userConfDir, _ := os.UserConfigDir()
		readConfigFile(filepath.Join(userConfDir, "geneos.json"), &c)
		printConfigJSON(c)
		return
	}

	// loop instances - parse the args again and load/print the config,
	// but allow for RC files again
	var cs []Instance
	for _, name := range names {
		for _, c := range New(ct, name) {
			if err = loadConfig(c, false); err != nil {
				log.Println(Type(c), Name(c), "cannot load configuration")
				continue
			}
			if c != nil {
				cs = append(cs, c)
			}
		}
	}
	printConfigJSON(cs)

	return
}

func printConfigJSON(Config interface{}) (err error) {
	if buffer, err := json.MarshalIndent(Config, "", "    "); err == nil {
		log.Printf("%s\n", buffer)
	}

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
func setCommand(ct ComponentType, names []string) (err error) {
	if len(names) == 0 {
		return os.ErrInvalid
	}

	// read the cofig into a struct, make changes, then save it out again,
	// to sanitise the contents - or generate an error
	switch names[0] {
	case "global":
		return setConfig(globalConfig, names[1:])
	case "user":
		userConfDir, _ := os.UserConfigDir()
		return setConfig(filepath.Join(userConfDir, "geneos.json"), names[1:])
	}

	// check if all args have an '=' - if so do the same as "set user"
	eqs := len(names)
	for _, arg := range names {
		if strings.Contains(arg, "=") {
			eqs--
		}
	}
	if eqs == 0 {
		userConfDir, _ := os.UserConfigDir()
		setConfig(filepath.Join(userConfDir, "geneos.json"), names)
		return
	}

	// components - parse the args again and load/print the config,
	// but allow for RC files again
	//
	// consume component names, stop at first parameter, error out if more names?
	var cs []Instance
	var setFlag bool

	for _, name := range names {
		if strings.Contains(name, "=") {
			s := strings.SplitN(name, "=", 2)
			// loop through all provided components, set the parameter(s)
			for _, c := range cs {
				setField(c, s[0], s[1])
				setFlag = true
			}
			continue
		}

		// if params found, stop if another component found
		if setFlag {
			log.Println("found")
			// error out
			break
		}

		for _, c := range New(ct, name) {
			// migration required to set values
			if err = loadConfig(c, true); err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				continue
			}
			if c != nil {
				cs = append(cs, c)
			}
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
		err = fmt.Errorf("cannot create %q: %s", file, errors.Unwrap(err))
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
func disableCommand(ct ComponentType, args []string) (err error) {
	return loopCommand(disable, ct, args)
}

func disable(c Instance) (err error) {
	if isDisabled(c) {
		return fmt.Errorf("already disabled")
	}

	if err = stop(c); err != nil && err != ErrProcNotExist {
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
func enableCommand(ct ComponentType, args []string) (err error) {
	return loopCommand(enable, ct, args)
}

func enable(c Instance) (err error) {
	d := filepath.Join(Home(c), Type(c).String()+disableExtension)
	if err = os.Remove(d); err == nil || errors.Is(err, os.ErrNotExist) {
		err = start(c)
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
func renameCommand(ct ComponentType, args []string) (err error) {
	if len(args) != 2 {
		return ErrInvalidArgs
	}
	to := New(ct, args[1])[0]
	if err = loadConfig(to, false); err == nil {
		return fmt.Errorf(args[1], "already exists")
	}
	from := New(ct, args[0])[0]
	if err = loadConfig(from, true); err != nil {
		log.Println(Type(from), Name(from), "cannot load configuration")
	}
	pid, err := findProc(from)
	if pid != 0 {
		return ErrProcExists
	}

	return ErrNotSupported
}

// also special - each component must be an exact match
// do we want a disable then delete protection?
func deleteCommand(ct ComponentType, args []string) (err error) {
	return ErrNotSupported
}
