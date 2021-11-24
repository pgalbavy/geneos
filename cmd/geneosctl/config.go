package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// process config files

func loadConfig(c Component) (cmd *exec.Cmd, env []string) {
	t := Type(c).String()

	wd := filepath.Join(RootDir(Type(c)), Name(c))
	if err := os.Chdir(wd); err != nil {
		log.Println("cannot chdir() to", wd)
		return
	}

	// load the JSON config file is available, otherwise load
	// the "legacy" .rc file and try to write out a JSON file
	// for later re-use
	jsonFile, err := os.ReadFile(t + ".json")
	if err == nil {
		json.Unmarshal(jsonFile, &c)
	} else {
		env = convertOldConfig(c, Name(c))
	}

	// build command line and env vars
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "/bin/bash"
	}

	// XXX abstract this stuff away
	binary := filepath.Join(getStringWithPrefix(c, "Bins"), getStringWithPrefix(c, "Base"), getString(c, "BinSuffix"))

	// XXX find common envs - JAVA_HOME etc.
	env = append(env, "LD_LIBRARY_PATH="+getStringWithPrefix(c, "Libs"))

	var args, extraenv []string

	// XXX args and env vary depending on Component type - the below is for Gateway
	// this should be pushed out to each compoent's own file
	switch Type(c) {
	case Gateway:
		args, extraenv = gatewayCmd(c)
	case Netprobe:
		args, extraenv = netprobeCmd(c)
	case Licd:
		args, extraenv = licdCmd(c)
	default:
	}

	args = append(args, getStringsWithPrefix(c, "Opts")...)
	env = append(env, extraenv...)
	cmd = exec.Command(binary, args...)

	return
}

func convertOldConfig(c Component, name string) (env []string) {
	t := Type(c).String()
	prefix := strings.Title(t[0:4])

	rcFile, err := os.Open(t + ".rc")
	if err != nil {
		log.Println("cannot open ", t, ".rc")
		return
	}
	defer rcFile.Close()

	wd := filepath.Join(RootDir(Type(c)), name)
	log.Printf("loading config from %s/%s.rc", wd, t)

	confs := make(map[string]string)
	scanner := bufio.NewScanner(rcFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		s := strings.SplitN(line, "=", 2)
		if len(s) != 2 {
			log.Println("config line format incorrect:", line)
			return
		}
		key, value := s[0], s[1]
		value = strings.Trim(value, "\"")
		confs[key] = value
	}

	// log.Printf("defaults: %+v\n", c)
	for k, v := range confs {
		switch k {
		case prefix + "Opts":
			setFields(c, prefix+"Opts", strings.Fields(v))
		case "BinSuffix":
			setField(c, k, v)
		default:
			if strings.HasPrefix(k, prefix) {
				setField(c, k, v)
			} else {
				// set env var
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	WriteConfig(c)
	return
}

func WriteConfig(c Component) {
	home := Home(c)
	t := Type(c).String()

	j, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		log.Println("json marshal failed:", err)
	} else {
		log.Printf("%s\n", string(j))
		err = os.WriteFile(filepath.Join(home, t+".json"), j, 0666)
		if err != nil {
			log.Println("cannot write JSON config file:", err)
		}
	}
}
