package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// The Command type contains the standard functions and help text for each command. Each command adds it's
// own in an init() function to the global commands map
type Command struct {
	// The main work function of the command. It accepts a Component (which can be None), a slice of arguments
	// and a separate clice of "parameters". See ParseArgs() and ParseFlags() to see why these are separate.
	Function func(Component, []string, []string) error
	// Optional function to parse any command flags after the command name. Given a slice of arguments process any
	// flags and return the unprocessed args. This allows each command to have it's own command line options after
	// the command name but before all the other arguments and parameters, e.g. "geneos logs -f example"
	ParseFlags func(string, []string) []string
	// Optional function to parse arguments. Given the remaining args after ParseFlags is done evaluate if there
	// is a Component and then separate command args from optional parameters. Any args that do not match
	// instance names are left on the params slice. It is up to the command
	ParseArgs func([]string) (Component, []string, []string)
	// Command Syntax
	CommandLine string
	// Short description
	Summary string
	// More detailed help
	Description string // details
}

// The Commands type is a map of command text (as a string) to a Command structure
type Commands map[string]Command

// return a single slice of all instances, ordered and grouped
// configuration are not loaded, just the defaults ready for overlay
func allInstances() (confs []Instances) {
	for _, ct := range realComponentTypes() {
		for _, remote := range allRemotes() {
			confs = append(confs, ct.instancesOfComponent(remote.Name())...)
		}
	}
	return
}

// return a slice of instancesOfComponent for a given Component
func (ct Component) instancesOfComponent(remote string) (confs []Instances) {
	for _, name := range ct.instanceDirsForComponent(remote) {
		confs = append(confs, ct.newComponent(name)...)
	}
	return
}

// return a slice of component types that exist for this name
func findInstances(name string) (cts []Component) {
	local, remote := splitInstanceName(name)
	for _, t := range realComponentTypes() {
		for _, dir := range t.instanceDirsForComponent(remote) {
			// for case insensitive match change to EqualFold here
			ldir, _ := splitInstanceName(dir)
			if filepath.Base(ldir) == local {
				cts = append(cts, t)
			}
		}
	}
	return
}

// loadConfig will load the JSON config file is available, otherwise
// try to load the "legacy" .rc file and optionally write out a JSON file
// for later re-use, while renaming .rc file as a backup
func loadConfig(c Instances, update bool) (err error) {
	baseconf := filepath.Join(c.Home(), c.Type().String())
	j := baseconf + ".json"

	if err = readConfigFile(c.Location(), j, &c); err == nil {
		// return if NO error, else drop through
		return
	}
	if err = readRCConfig(c); err != nil {
		return
	}
	if update {
		if err = writeInstanceConfig(c); err != nil {
			logError.Println("failed to wrtite config file:", err)
			return
		}
		if err = renameFile(c.Location(), baseconf+".rc", baseconf+".rc.orig"); err != nil {
			logError.Println("failed to rename old config:", err)
		}
		logDebug.Println(c.Type(), c.Name(), "migrated to JSON config")
	}

	return
}

// buildCmd gathers the path to the binary, arguments and any environment variables
// for an instance and returns an exec.Cmd, almost ready for execution. Callers
// will add more details such as working directories, user and group etc.
func buildCmd(c Instances) (cmd *exec.Cmd, env []string) {
	binary := getString(c, c.Prefix("Exec"))

	args, env := c.Command()

	opts := strings.Fields(getString(c, c.Prefix("Opts")))
	args = append(args, opts...)
	// XXX find common envs - JAVA_HOME etc.
	env = append(env, getSliceStrings(c, "Env")...)
	env = append(env, "LD_LIBRARY_PATH="+getString(c, c.Prefix("Libs")))
	cmd = exec.Command(binary, args...)

	return
}

// save off extra env too
// XXX - scan file line by line, protect memory
func readRCConfig(c Instances) (err error) {
	rcdata, err := readFile(c.Location(), filepath.Join(c.Home(), c.Type().String()+".rc"))
	if err != nil {
		return
	}
	logDebug.Printf("loading config from %s/%s.rc", c.Home(), c.Type())

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
			if strings.HasPrefix(k, c.Prefix("")) {
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
