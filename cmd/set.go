/*
Copyright © 2022 Peter Galbavy <peter@wonderland.org>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/component"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
)

// setCmd represents the set command
var setCmd = &cobra.Command{
	Use:   "set [TYPE] [NAME...] KEY=VALUE [KEY=VALUE...]",
	Short: "Set instance configuration parameters",
	Long: `Set configuration item values in global, user, or for a specific
	instance.
	
	Special Names:
	
	To set environment variables for an instance use the key Env and the
	value var=value. Each new var=value is additive or overwrites an existing
	entry for 'var', e.g.
	
		geneos set netprobe localhost Env=JAVA_HOME=/usr/lib/jre
		geneos set netprobe localhost Env=ORACLE_HOME=/opt/oracle
	
	To remove an environment variable prefix the name with a hyphen '-', e.g.
	
		geneos set netprobe localhost Env=-JAVA_HOME
	
	To add an include file to an auto-generated gateway use a similar syntax to the above, but in the form:
	
		geneos set gateway gateway1 Includes=100:path/to/include.xml
		geneos set gateway gateway1 Includes=-100
	
	Then rebuild the configuration as required.
	
	Other special names include Gateways for a comma separated list of host:port values for Sans,
	Attributes as name=value pairs again for Sans and Types a comma separated list of Types for Sans.
	Variables (for San config templates) cannot be set from the command line at this time.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("set called")
	},
}

func init() {
	rootCmd.AddCommand(setCmd)
}

// components - parse the args again and load/print the config,
// but allow for RC files again
//
// consume component names, stop at first parameter, error out if more names
func commandSet(ct component.ComponentType, args []string, params []string) (err error) {
	var instances []instance.Instance

	logDebug.Println("args", args, "params", params)

	if len(args) == 0 && len(params) == 0 {
		return ErrInvalidArgs
	}

	if ct != component.None && len(args) == 0 {
		// if all args have no become params (e.g. 'set gateway X=Y') then reprocess args here
		args = instance.FindNames(host.ALL, ct)
	} else if len(args) == 0 || args[0] == "user" {
		userConfDir, _ := os.UserConfigDir()
		return writeConfigParams(filepath.Join(userConfDir, "geneos.json"), params)
	} else if args[0] == "global" {
		return writeConfigParams(globalConfig, params)
	}

	// loop through named instances
	for _, arg := range args {
		instances = append(instances, instance.FindInstances(ct, arg)...)
	}

	for _, arg := range params {
		s := strings.SplitN(arg, "=", 2)
		if len(s) != 2 {
			logError.Printf("ignoring %q %s", arg, ErrInvalidArgs)
			continue
		}
		k, v := s[0], s[1]

		// loop through all provided instances, set the parameter(s)
		for _, c := range instances {
			for _, vs := range strings.Split(v, ",") {
				if err = setValue(c, k, vs); err != nil {
					log.Printf("%s: cannot set %q", c, k)
				}
			}
		}
	}

	// now loop through the collected results and write out
	for _, c := range instances {
		if err = migrateConfig(c); err != nil {
			logError.Fatalln("cannot migrate existing .rc config to set values in new .json configration file:", err)
		}
		if err = writeInstanceConfig(c); err != nil {
			logError.Fatalln(err)
		}
	}

	return
}

var pluralise = map[string]string{
	"Gateway":   "s",
	"Attribute": "s",
	"Type":      "s",
	"Include":   "s",
}

func setValue(c Instance, tag, v string) (err error) {
	defaults := map[string]string{
		"Includes": "100",
		"Gateways": "7039",
	}

	if pluralise[tag] != "" {
		tag = tag + pluralise[tag]
	}

	switch tag {
	// make this list dynamic
	case "Includes", "Gateways":
		var remove bool
		e := strings.SplitN(v, ":", 2)
		if strings.HasPrefix(e[0], "-") {
			e[0] = strings.TrimPrefix(e[0], "-")
			remove = true
		}
		if remove {
			err = setStructMap(c, tag, e[0], "")
			if err != nil {
				logDebug.Printf("%s delete %v[%v] failed, %s", c, tag, e[0], err)
			}
		} else {
			val := defaults[tag]
			if len(e) > 1 {
				val = e[1]
			} else {
				// XXX check two values and first is a number
				logDebug.Println("second value missing after ':', using default", val)
			}
			err = setStructMap(c, tag, e[0], val)
			if err != nil {
				logDebug.Printf("%s set %v[%v]=%v failed, %s", c, tag, e[0], val, err)
			}
		}
	case "Attributes":
		var remove bool
		e := strings.SplitN(v, "=", 2)
		if strings.HasPrefix(e[0], "-") {
			e[0] = strings.TrimPrefix(e[0], "-")
			remove = true
		}
		// '-name' or 'name=' remove the attribute
		if remove || len(e) == 1 {
			err = setStructMap(c, tag, e[0], "")
			if err != nil {
				logDebug.Printf("%s delete %v[%v] failed, %s", c, tag, e[0], err)
			}
		} else {
			err = setStructMap(c, tag, e[0], e[1])
			if err != nil {
				logDebug.Printf("%s set %v[%v]=%v failed, %s", c, tag, e[0], e[1], err)
			}
		}
	case "Env", "Types":
		var remove bool
		slice := c.V.getSliceStrings(tag)
		e := strings.SplitN(v, "=", 2)
		if strings.HasPrefix(e[0], "-") {
			e[0] = strings.TrimPrefix(e[0], "-")
			remove = true
		}
		anchor := "="
		if remove && strings.HasSuffix(e[0], "*") {
			// wildcard removal (only)
			e[0] = strings.TrimSuffix(e[0], "*")
			anchor = ""
		}
		var exists bool
		// transfer items to new slice as removing items in a loop
		// does random things
		var newslice []string
		for _, n := range slice {
			if strings.HasPrefix(n, e[0]+anchor) {
				if !remove {
					// replace with new value
					newslice = append(newslice, v)
					exists = true
				}
			} else {
				// copy existing
				newslice = append(newslice, n)
			}
		}
		// add a new item rather than update or remove
		if !exists && !remove {
			newslice = append(newslice, v)
		}
		if err = c.V.Set(tag, newslice); err != nil {
			logDebug.Printf("%s set %s=%s failed, %s", c, tag, newslice, err)
		}
	case "Variables", "Variable", "Var":
		// syntax: "[TYPE:]NAME=VALUE" - TYPE defaults to string
		// TYPE must match what's in XML, or just pass straight through anyway
		// lowercase?
		// NAME use unchanged, case sensitive
		//
		// Support only the following types:
		//   activeTime, boolean, double, integer, string, externalConfigFile
		//

		// tag = "Var", v = [TYPE]:KEY=VALUE, TYPE default = string

		var remove bool
		if strings.HasPrefix(v, "-") {
			v = strings.TrimPrefix(v, "-")
			err = setStructMap(c, tag, v, "")
			if err != nil {
				logDebug.Printf("%s delete %v[%v] failed, %s", c, tag, v, err)
			}
			return
		}

		var t, key, value string

		e := strings.SplitN(v, ":", 2)
		if len(e) == 1 {
			t = "string"
			s := strings.SplitN(e[0], "=", 2)
			if len(s) == 1 && !remove {
				logError.Printf("invalid format for variable: %q", v)
				return ErrInvalidArgs
			}
			key = s[0]
			value = s[1]
		} else {
			t = e[0]
			s := strings.SplitN(e[1], "=", 2)
			if len(s) == 1 && !remove {
				logError.Printf("invalid format for variable: %q", v)
				return ErrInvalidArgs
			}
			key = s[0]
			value = s[1]
		}

		// XXX check types here - e[0] options type, default string
		var validtypes map[string]string = map[string]string{
			"string":             "",
			"integer":            "",
			"double":             "",
			"boolean":            "",
			"activeTime":         "",
			"externalConfigFile": "",
		}
		if _, ok := validtypes[t]; !ok {
			logError.Printf("invalid type %q for variable", t)
			return ErrInvalidArgs
		}
		val := t + ":" + value
		err = setStructMap(c, tag, key, val)
		if err != nil {
			logDebug.Printf("%s set %v[%v]=%v failed, %s", c, tag, e[0], val, err)
		}

	default:
		if err = c.V.Set(tag, v); err != nil {
			logDebug.Printf("%s set %s=%s failed, %s", c, tag, v, err)
		}
	}
	return
}

func writeConfigParams(filename string, params []string) (err error) {
	var c GlobalSettings
	// ignore err - config may not exist, but that's OK
	_ = readLocalConfigFile(filename, &c)
	// change here
	if len(c) == 0 {
		c = make(GlobalSettings)
	}
	for _, set := range params {
		// skip all non '=' args
		if !strings.Contains(set, "=") {
			continue
		}
		s := strings.SplitN(set, "=", 2)
		k, v := s[0], s[1]
		c[Global(k)] = v
	}

	// fix breaking change
	if oldhome, ok := c["ITRSHome"]; ok {
		if newhome, ok := c["Geneos"]; !ok || newhome == "" {
			c["Geneos"] = oldhome
		}
		delete(c, "ITRSHome")
	}

	// XXX fix permissions assumptions here
	if filename == globalConfig {
		return host.LOCAL.writeConfigFile(filename, "root", 0664, c)
	}
	return host.LOCAL.writeConfigFile(filename, "", 0664, c)
}
