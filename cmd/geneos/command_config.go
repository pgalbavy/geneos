package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var globalConfig = "/etc/geneos/geneos.json"

type ConfigType struct {
	Root string `json:"root,omitempty"`
}

var Config ConfigType

func init() {
	commands["config"] = commandConfig

	readConfigFile(globalConfig, &Config)
	userConfDir, _ := os.UserConfigDir()
	readConfigFile(filepath.Join(userConfDir, "geneos.json"), &Config)

	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		Config.Root = h
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
		return migrateConfig(args[1:])
	case "show", "get":
		return showConfig(args[1:])
	case "set":
		return setConfig(args[1:])
	default:
		return fmt.Errorf("unknown config command option: %q", args[0])
	}

	return
}

func migrateConfig(args []string) (err error) {
	return
}

func showConfig(args []string) (err error) {
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
	for _, name := range names {
		for _, c := range New(comp, name) {
			err = loadConfig(c, false)
			if err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				continue
			}
			printConfigJSON(c)
		}
	}

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

func setConfig(args []string) (err error) {
	key, value := args[1], args[2]
	log.Printf("before: %+v\n", Config)
	setField(&Config, key, value)
	log.Printf("after: %+v\n", Config)

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
func writeConfigFile(file string, config ConfigType) (err error) {
	// marshal
	buffer, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return
	}

	// atomic-ish write
	dir, name := filepath.Split(file)
	f, err := os.CreateTemp(dir, name)
	defer os.Remove(f.Name())
	_, err = f.Write(buffer)
	if err != nil {
		return
	}
	err = os.Rename(f.Name(), file)
	if err != nil {
		return
	}
	return
}
