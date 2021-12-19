package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Command struct {
	Function     func(ComponentType, []string, []string) error
	ParseArgs    func([]string) (ComponentType, []string, []string)
	CommandLine  string // one line command syntax
	Descrtiption string // details
}

type Commands map[string]Command

// return a single slice of all instances, ordered and grouped
func allInstances() (confs []Instance) {
	for _, ct := range componentTypes() {
		confs = append(confs, instances(ct)...)
	}
	return
}

func instances(ct ComponentType) (confs []Instance) {
	for _, name := range instanceDirs(ct) {
		confs = append(confs, NewComponent(ct, name)...)
	}
	return
}

func findInstances(name string) (cts []ComponentType) {
	for _, t := range componentTypes() {
		compdirs := instanceDirs(t)
		for _, dir := range compdirs {
			// for case insensitive match change to EqualFold here
			// but also in NewInstance()
			if filepath.Base(dir) == name {
				cts = append(cts, t)
			}
		}
	}
	return
}

func loadConfig(c Instance, update bool) (err error) {
	// load the JSON config file is available, otherwise load
	// the "legacy" .rc file and optionally write out a JSON file
	// for later re-use, while renaming .rc file
	baseconf := filepath.Join(Home(c), Type(c).String())
	j := baseconf + ".json"

	if err = readConfigFile(j, &c); err == nil {
		return
	}
	if err = readRCConfig(c); err != nil {
		return
	}
	if update {
		// select if we want this or not
		if err = writeConfigFile(baseconf+".json", c); err == nil {
			// preserves ownership and permnissions
			os.Rename(baseconf+".rc", baseconf+".rc.orig")
		}
		log.Println(Type(c), Name(c), "migrated to JSON config")
	}

	return
}

func buildCommand(c Instance) (cmd *exec.Cmd, env []string) {
	binary := filepath.Join(getString(c, Prefix(c)+"Bins"),
		getString(c, Prefix(c)+"Base"),
		getString(c, "BinSuffix"))

	// test binary for access
	if _, err := os.Stat(binary); err != nil {
		log.Println("binary not found:", binary)
		return
	}

	var args []string

	// XXX args and env vary depending on Component type - the below is for Gateway
	// this should be pushed out to each compoent's own file
	switch Type(c) {
	case Gateways:
		args, env = gatewayCommand(c)
	case Netprobes:
		args, env = netprobeCommand(c)
	case Licds:
		args, env = licdCommand(c)
	default:
		return
	}

	opts := strings.Fields(getString(c, Prefix(c)+"Opts"))
	args = append(args, opts...)
	// XXX find common envs - JAVA_HOME etc.
	env = append(env, getSliceStrings(c, "Env")...)
	env = append(env, "LD_LIBRARY_PATH="+getString(c, Prefix(c)+"Libs"))
	cmd = exec.Command(binary, args...)

	return
}

// save off extra env too
// XXX - scan file line by line, protect memory
func readRCConfig(c Instance) (err error) {
	rcdata, err := os.ReadFile(filepath.Join(Home(c), Type(c).String()+".rc"))
	if err != nil {
		return
	}
	DebugLog.Printf("loading config from %s/%s.rc", Home(c), Type(c))

	confs := make(map[string]string)

	rcFile := bytes.NewBuffer(rcdata)
	scanner := bufio.NewScanner(rcFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		s := strings.SplitN(line, "=", 2)
		if len(s) != 2 {
			return ErrInvalidArgs
		}
		key, value := s[0], s[1]
		value = strings.Trim(value, "\"")
		confs[key] = value
	}

	var env []string
	// log.Printf("defaults: %+v\n", c)
	for k, v := range confs {
		switch k {
		case "BinSuffix":
			if err = setField(c, k, v); err != nil {
				return
			}
		default:
			if strings.HasPrefix(k, Prefix(c)) {
				if err = setField(c, k, v); err != nil {
					return
				}
			} else {
				// set env var
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	return setFieldSlice(c, "Env", env)
}
