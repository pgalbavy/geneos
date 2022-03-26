package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// The Command type contains the standard functions and help text for each command. Each command adds it's
// own in an init() function to the global commands map
type Command struct {
	Name string
	// The main work function of the command. It accepts a Component (which can be None), a slice of arguments
	// and a separate clice of "parameters". See ParseArgs() and ParseFlags() to see why these are separate.
	Function func(Component, []string, []string) error
	// Function to parse any command flags after the command name. Given a slice of arguments process any
	// flags and return the unprocessed args. This allows each command to have it's own command line options after
	// the command name but before all the other arguments and parameters, e.g. "geneos logs -f example"
	//
	// This is now called from inside ParseArgs() below
	ParseFlags func(string, []string) []string
	// Function to parse arguments. Given the remaining args after ParseFlags is done evaluate if there
	// is a Component and then separate command args from optional parameters. Any args that do not match
	// instance names are left on the params slice. It is up to the command
	ParseArgs     func(Command, []string) (Component, []string, []string)
	Wildcard      bool
	ComponentOnly bool
	// Command Syntax
	CommandLine string
	// Short description
	Summary string
	// More detailed help
	Description string // details
}

// The Commands type is a map of command text (as a string) to a Command structure
type Commands map[string]Command

func RegsiterCommand(cmd Command) {
	if cmd.Name == "" {
		return
	}

	commands[cmd.Name] = cmd
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

// read an old style .rc file. parameters are one-per-line and are key=value
// any keys that do not match the component prefix or the special
// 'BinSuffix' are terated as environment variables
//
// No processing of shell variables. should there be?
func readRCConfig(c Instances) (err error) {
	rcdata, err := c.Remote().readFile(InstanceFileWithExt(c, "rc"))
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
			return fmt.Errorf("invalid line (must be key=value) %q: %w", line, ErrInvalidArgs)
		}
		key, value := s[0], s[1]
		value = strings.Trim(value, "\"")
		confs[key] = value
	}

	var env []string
	for k, v := range confs {
		if k == "BinSuffix" {
			if err = setField(c, k, v); err != nil {
				return
			}
			continue
		}
		// this doesn't work if Prefix is empty
		if strings.HasPrefix(k, c.Prefix("")) {
			if err = setField(c, k, v); err != nil {
				return
			}
		} else {
			// set env var
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return setFieldSlice(c, "Env", env)
}
