package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Command struct {
	Function     func(ComponentType, []string) error
	ParseArgs    func([]string) (ComponentType, []string)
	Descrtiption string
}

type Commands map[string]Command

// process config file(s)

func allInstances() (confs map[ComponentType][]Instance) {
	confs = make(map[ComponentType][]Instance)
	for _, ct := range ComponentTypes() {
		confs[ct] = instances(ct)
	}
	return
}

func instances(ct ComponentType) (confs []Instance) {
	for _, name := range InstanceDirs(ct) {
		confs = append(confs, New(ct, name)...)
	}
	return
}

func findInstances(name string) (cts []ComponentType) {
	for _, t := range ComponentTypes() {
		compdirs := InstanceDirs(t)
		for _, dir := range compdirs {
			if filepath.Base(dir) == name {
				cts = append(cts, t)
			}
		}
	}
	return
}

func loadConfig(c Instance, update bool) (err error) {
	// load the JSON config file is available, otherwise load
	// the "legacy" .rc file and try to write out a JSON file
	// for later re-use
	baseconf := filepath.Join(Home(c), Type(c).String())
	j := baseconf + ".json"
	jsonFile, err := os.ReadFile(j)
	if err == nil {
		if err = json.Unmarshal(jsonFile, &c); err != nil {
			return
		}
	} else {
		if err = readRCConfig(c); err != nil {
			return
		}
		if update {
			// select if we want this or not
			if err = writeConfigFile(baseconf+".json", c); err == nil {
				os.Rename(baseconf+".rc", baseconf+".rc.orig")
			}
			log.Println(Type(c), Name(c), "migrated to JSON config")
		}
	}

	return
}

func buildCommand(c Instance) (cmd *exec.Cmd, env []string) {
	// XXX abstract this stuff away
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
	case Gateway:
		args, env = gatewayCmd(c)
	case Netprobe:
		args, env = netprobeCmd(c)
	case Licd:
		args, env = licdCmd(c)
	default:
		//
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
func readRCConfig(c Instance) (err error) {
	rcdata, err := os.ReadFile(filepath.Join(Home(c), Type(c).String()+".rc"))
	if err != nil {
		return
	}

	wd := filepath.Join(InstanceDir(Type(c)), Name(c))
	DebugLog.Printf("loading config from %s/%s.rc", wd, Type(c))

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
			setField(c, k, v)
		default:
			if strings.HasPrefix(k, Prefix(c)) {
				setField(c, k, v)
			} else {
				// set env var
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	setFieldSlice(c, "Env", env)

	return
}
