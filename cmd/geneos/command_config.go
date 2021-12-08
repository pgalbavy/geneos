package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
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
	DownloadURL string `json:",omitempty"`

	// Username to start components if not explicitly defined
	// and we are running with elevated privileges
	//
	// When running as a normal user this is unused and
	// we simply test a defined user against the running user
	//
	// default is owner of ITRSHome
	DefaultUser string `json:",omitempty"`
}

var Config ConfigType

func init() {
	commands["init"] = Command{initCommand, parseArgs, "initialise"}
	commands["migrate"] = Command{migrateCommand, parseArgs, "migrate"}
	commands["revert"] = Command{revertCommand, parseArgs, "revert"}
	commands["show"] = Command{showCommand, parseArgs, "show"}
	commands["set"] = Command{setCommand, parseArgs, "set"}

	commands["disable"] = Command{disableCommand, parseArgs, "disable"}
	commands["enable"] = Command{enableCommand, parseArgs, "enable"}
	commands["rename"] = Command{renameCommand, parseArgs, "rename"}
	commands["delete"] = Command{deleteCommand, parseArgs, "delete"}

	readConfigFile(globalConfig, &Config)
	userConfDir, _ := os.UserConfigDir()
	readConfigFile(filepath.Join(userConfDir, "geneos.json"), &Config)

	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		Config.ITRSHome = h
	}

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
func initCommand(comp ComponentType, names []string) (err error) {
	return ErrNotSupported
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

func migrateCommand(comp ComponentType, names []string) (err error) {
	if len(names) == 0 {
		return os.ErrInvalid
	}

	// do components - parse the args again and load/print the config,
	// but allow for RC files again
	for _, name := range names {
		for _, c := range New(comp, name) {
			// passing true here migrates the RC file, doing nothing ir already
			// in JSON format
			err = loadConfig(c, true)
			if err != nil {
				log.Println(Type(c), Name(c), "cannot migrate configuration", err)
				continue
			}
		}
	}

	return
}

// rename rc.orig to rc, remove JSON, return
//
func revertCommand(comp ComponentType, names []string) (err error) {
	if len(names) == 0 {
		return os.ErrInvalid
	}

	// do compoents - parse the args again and load/print the config,
	// but allow for RC files again
	for _, name := range names {
		for _, c := range New(comp, name) {
			// load a config, following normal logic, first
			err = loadConfig(c, false)
			if err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				continue
			}
			baseconf := filepath.Join(Home(c), Type(c).String())

			// if *.rc file exists, remove rc.orig+JSON, continue
			_, err := os.Stat(baseconf + ".rc")
			if err == nil {
				// ignore errors
				if os.Remove(baseconf+".rc.orig") == nil || os.Remove(baseconf+".json") == nil {
					log.Println(Type(c), Name(c), "removed extra config file(s)")
				}
				continue
			}

			err = os.Rename(baseconf+".rc.orig", baseconf+".rc")
			if err != nil {
				log.Println(Type(c), Name(c), err)
				continue
			}

			err = os.Remove(baseconf + ".json")
			if err != nil {
				log.Println(Type(c), Name(c), err)
				continue
			}

			log.Println(Type(c), Name(c), "reverted to RC config")
		}
	}

	return
}

func showCommand(comp ComponentType, names []string) (err error) {
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

	// do compoents - parse the args again and load/print the config,
	// but allow for RC files again
	var cs []Component
	for _, name := range names {
		for _, c := range New(comp, name) {
			err = loadConfig(c, false)
			if err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
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
	buffer, err := json.MarshalIndent(Config, "", "    ")
	if err != nil {
		return
	}

	log.Printf("%s\n", buffer)
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
// geneos config set gateway wonderland GatePort=8888
// geneos config global ITRSHome=/opt/geneos
//
// quoting is left to the shell rules and the setting is just split on the first '='
// non '=' args are taken as other names?
//
// What is read only? Name, others?
//
func setCommand(comp ComponentType, names []string) (err error) {
	if len(names) == 0 {
		return os.ErrInvalid
	}

	// read the cofig into a struct, make changes, then save it out again,
	// to sanitise the contents - or generate an error
	switch names[0] {
	case "global":
		setConfig(globalConfig, names[1:])
		return
	case "user":
		userConfDir, _ := os.UserConfigDir()
		setConfig(filepath.Join(userConfDir, "geneos.json"), names[1:])
		return
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
	var cs []Component
	var setFlag bool

	log.Println("args:", names)
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

		for _, c := range New(comp, name) {
			// migration required to set values
			err = loadConfig(c, true)
			if err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
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

// set envs?
// geneos set gateway test Env+X=Y Env-Z Env=A=B
//
func setConfig(filename string, args []string) (err error) {
	var c ConfigType
	readConfigFile(filename, &c)
	// change here
	for _, set := range args {
		// skip all non '=' args
		if !strings.Contains(set, "=") {
			continue
		}
		s := strings.SplitN(set, "=", 2)
		k, v := s[0], s[1]
		err = setField(&c, k, v)

	}
	writeConfigFile(filename, c)
	return
}

func readConfigFile(file string, config interface{}) (err error) {
	f, err := os.ReadFile(file)
	if err == nil {
		err = json.Unmarshal(f, &config)
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

	// atomic-ish write
	dir, name := filepath.Split(file)
	f, err := os.CreateTemp(dir, name)
	if err != nil {
		err = fmt.Errorf("cannot create %q: %s", file, errors.Unwrap(err))
		return
	}
	defer os.Remove(f.Name())
	_, err = fmt.Fprintln(f, string(buffer))
	if err != nil {
		return
	}

	// if we've been run as root then try to change the new
	// file to the same user as the component. If this is
	// not a component config file then do nothing (as there
	// is no prefix or User config field)
	if superuser {
		username := getString(config, Prefix(config)+"User")
		if username != "" {
			u, err := user.Lookup(username)
			if err != nil {
				return err
			}
			uid, _ := strconv.Atoi(u.Uid)
			gid, _ := strconv.Atoi(u.Gid)
			err = f.Chown(uid, gid)
			if err != nil {
				return err
			}
		}
	}
	// XXX - these should not be hardwired
	err = f.Chmod(0664)
	if err != nil {
		return
	}
	err = os.Rename(f.Name(), file)
	if err != nil {
		return
	}
	return
}

const disableExtension = ".disabled"

// geneos disable gateway example ...
// stop if running first
// if run as root, the disable file is owned by root too and
// only root can remove it?
func disableCommand(comp ComponentType, args []string) (err error) {
	return loopCommand(disable, comp, args)
}

func disable(c Component) (err error) {
	if isDisabled(c) {
		return fmt.Errorf("already disabled")
	}

	err = stop(c)
	if err != nil && err != ErrProcNotExist {
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

	err = f.Chown(uid, gid)
	if err != nil {
		os.Remove(f.Name())
		return
	}
	return
}

// simpler than disable, just try to remove the flag file
// we do also start the component(s)
func enableCommand(comp ComponentType, args []string) (err error) {
	return loopCommand(enable, comp, args)
}

func enable(c Component) (err error) {
	d := filepath.Join(Home(c), Type(c).String()+disableExtension)
	err = os.Remove(d)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		err = start(c)
		return err
	}
	return
}

func isDisabled(c Component) bool {
	d := filepath.Join(Home(c), Type(c).String()+disableExtension)
	f, err := os.Stat(d)
	// an error tends to mean the disabled file is not
	// visible, so assume it's not disabled - other things
	// may be wrong but...
	if err != nil {
		return false
	}
	if err == nil && f.Mode().IsRegular() {
		return true
	}
	return false
}

// this is special and the normal loopCommand will not work
// 'geneos rename gateway abc xyz'
func renameCommand(comp ComponentType, args []string) (err error) {
	return ErrNotSupported
}

// also special - each component must be an exact match
// do we want a disable then delete protection?
func deleteCommand(comp ComponentType, args []string) (err error) {
	return ErrNotSupported
}
