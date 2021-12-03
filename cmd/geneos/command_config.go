package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

var globalConfig = "/etc/geneos/geneos.json"

type ConfigType struct {
	ITRSHome    string `json:",omitempty"`
	DownloadURL string `json:",omitempty"`
}

var Config ConfigType

func init() {
	commands["config"] = commandConfig

	readConfigFile(globalConfig, &Config)
	userConfDir, _ := os.UserConfigDir()
	readConfigFile(filepath.Join(userConfDir, "geneos.json"), &Config)

	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		Config.ITRSHome = h
	}

}

//
//
// geneos config get | set
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
func commandConfig(comp ComponentType, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("not enough parameters")
	}

	switch args[0] {
	case "migrate":
		return migrateCommand(args[1:])
	case "revert":
		return revertCommand((args[1:]))
	case "show", "get":
		return showCommand(args[1:])
	case "set":
		return setCommand(args[1:])
	default:
		return fmt.Errorf("unknown config command option: %q", args[0])
	}
}

func migrateCommand(args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("not enough args")
	}
	if args[0] == "global" || args[0] == "user" {
		return fmt.Errorf("migrate is only for components")
	}

	// do compoents - parse the args again and load/print the config,
	// but allow for RC files again
	comp, names := parseArgs(args)
	for _, name := range names {
		for _, c := range New(comp, name) {
			// passing true here migrates the RC file, doing nothing ir already
			// in JSON format
			err = loadConfig(c, true)
			if err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				continue
			}
		}
	}

	return
}

func revertCommand(args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("not enough args")
	}
	if args[0] == "global" || args[0] == "user" {
		return fmt.Errorf("migrate is only for components")
	}

	// do compoents - parse the args again and load/print the config,
	// but allow for RC files again
	comp, names := parseArgs(args)
	for _, name := range names {
		for _, c := range New(comp, name) {
			// passing true here migrates the RC file, doing nothing ir already
			// in JSON format
			err = loadConfig(c, false)
			if err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				continue
			}
			baseconf := filepath.Join(Home(c), Type(c).String())
			err = os.Rename(baseconf+".rc.orig", baseconf+".rc")
			if err != nil {
				log.Println("cannot revert", baseconf+".rc.orig", "to .rc:", err)
				continue
			}
			err = os.Remove(baseconf + ".json")
			if err != nil {
				log.Println("cannot remove", baseconf+".json:", err)
			}
		}
	}

	return
}

func showCommand(args []string) (err error) {
	// default to combined global + user config
	// allow overrides to show specific or components
	if len(args) == 0 {
		// special case "config show" for resolved settings
		printConfigJSON(Config)
		return
	}

	// read the cofig into a struct then print it out again,
	// to sanitise the contents - or generate an error
	switch args[0] {
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
	comp, names := parseArgs(args)
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
func setCommand(args []string) (err error) {
	if len(args) == 0 {
		err = fmt.Errorf("not enough args")
		return
	}

	// read the cofig into a struct, make changes, then save it out again,
	// to sanitise the contents - or generate an error
	switch args[0] {
	case "global":
		setConfig(globalConfig, args[1:])
		return
	case "user":
		userConfDir, _ := os.UserConfigDir()
		setConfig(filepath.Join(userConfDir, "geneos.json"), args[1:])
		return
	}

	// do compoents - parse the args again and load/print the config,
	// but allow for RC files again
	comp, names := parseArgs(args)
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
