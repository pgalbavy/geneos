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

func loadConfig(c Component, name string) (cmd *exec.Cmd, env []string) {
	t := Type(c).String()

	wd := filepath.Join(compRootDir(Type(c)), name)
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
		env = convertOldConfig(c, name)
	}

	// build command line and env vars
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "/bin/bash"
	}

	// XXX abstract this stuff away
	binary := filepath.Join(getStringWithPrefix(c, "Bins"), getStringWithPrefix(c, "Base"), getString(c, "BinSuffix"))
	resourcesDir := filepath.Join(getStringWithPrefix(c, "Bins"), getStringWithPrefix(c, "Base"), "resources")

	logFile := filepath.Join(getStringWithPrefix(c, "LogD"), name, getStringWithPrefix(c, "LogF"))
	setupFile := filepath.Join(getStringWithPrefix(c, "Home"), "gateway.setup.xml")

	// XXX find common envs - JAVA_HOME etc.
	env = append(env, "LD_LIBRARY_PATH="+getStringWithPrefix(c, "Libs"))

	var args []string
	// XXX args and env vary depending on Component type - the below is for Gateway
	switch Type(c) {
	case Gateway:
		args = []string{
			/* "-gateway-name",  */ name,
			"-setup-file", setupFile,
			"-resources-dir", resourcesDir,
			"-log", logFile,
			"-licd-host", getStringWithPrefix(c, "LicH"),
			"-licd-port", getIntWithPrefix(c, "LicP"),
			// "-port", getIntWithPrefix(c, "Port"),
		}
	case Netprobe:
		env = append(env, "LOGFILE="+logFile)
	default:
	}

	args = append(args, getStringFieldSlice(c, "Opts")...)
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

	wd := filepath.Join(compRootDir(Type(c)), name)
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
			setStringFieldSlice(c, prefix+"Opts", strings.Fields(v))
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

	j, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		log.Println("json marshal failed:", err)
	} else {
		log.Printf("%s\n", string(j))
		err = os.WriteFile(t+".json", j, 0666)
		if err != nil {
			log.Println("cannot write JSON config file:", err)
		}
	}
	return
}
