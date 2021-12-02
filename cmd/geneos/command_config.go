package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var globalConfig = "/etc/geneos/geneos.json"

type ConfigType struct {
	Root string `json:"root"`
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
func commandConfig(comp ComponentType, args []string) (err error) {
	var useglobal bool
	if len(args) == 0 {
		return fmt.Errorf("not enough parameters")
	}
	if args[0] == "global" {
		if len(args) == 1 {
			return fmt.Errorf("not enough parameters after global")
		}
		useglobal = true
		args = args[1:]
	}

	switch args[0] {
	case "set":
		key, value := args[1], args[2]
		log.Printf("before: %+v\n", Config)
		setField(&Config, key, value)
		log.Printf("after: %+v\n", Config)
	case "get":
	case "show":
		var buffer []byte
		buffer, err = json.MarshalIndent(Config, "", "    ")
		if err != nil {
			return
		}
		log.Println(string(buffer))
	default:
		return fmt.Errorf("unknown option %q", args[0])
	}

	_ = useglobal
	return
}

func readConfigFile(file string, config *ConfigType) (err error) {
	f, err := os.ReadFile(file)
	if err == nil {
		err = json.Unmarshal(f, &config)
	}
	return
}

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
