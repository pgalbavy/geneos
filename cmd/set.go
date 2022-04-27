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
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
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
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := processArgs(cmd)
		return commandSet(ct, args, params)
	},
}

func init() {
	rootCmd.AddCommand(setCmd)

	setCmd.Flags().VarP(&setCmdEnvs, "env", "e", "Add an environment variable in the format NAME=VALUE")
	setCmd.Flags().VarP(&setCmdIncludes, "include", "i", "Add an include file in the format PRIORITY:PATH")
	setCmd.Flags().VarP(&setCmdGateways, "gateway", "g", "Add a gateway in the format NAME:PORT")
	setCmd.Flags().VarP(&setCmdAttributes, "attribute", "a", "Add an attribute in the format NAME=VALUE")
	setCmd.Flags().VarP(&setCmdTypes, "type", "t", "Add a gateway in the format NAME:PORT")
	setCmd.Flags().VarP(&setCmdVariables, "variable", "v", "Add a variable in the format [TYPE:]NAME=VALUE")
	setCmd.Flags().SortFlags = false
}

var setCmdIncludes = make(IncludeValues)
var setCmdGateways = make(GatewayValues)
var setCmdAttributes = make(NamedValues)
var setCmdEnvs = make(NamedValues)
var setCmdVariables = make(VarValues)
var setCmdTypes = TypeValues{}

func commandSet(ct *geneos.Component, args, params []string) error {
	return instance.ForAll(ct, setInstance, args, params)
}

func setInstance(c geneos.Instance, params []string) (err error) {
	logDebug.Println("c", c, "params", params)

	// walk through any flags passed
	setMaps(c)

	for _, arg := range params {
		s := strings.SplitN(arg, "=", 2)
		if len(s) != 2 {
			logError.Printf("ignoring %q %s", arg, ErrInvalidArgs)
			continue
		}
		k, v := s[0], s[1]

		// loop through all provided instances, set the parameter(s)
		for _, vs := range strings.Split(v, ",") {
			if err = setValue(c, k, vs); err != nil {
				log.Printf("%s: cannot set %q", c, k)
			}
		}
	}

	// now loop through the collected results and write out
	if err = instance.Migrate(c); err != nil {
		logError.Fatalln("cannot migrate existing .rc config to set values in new .json configration file:", err)
	}
	if err = instance.WriteConfig(c); err != nil {
		logError.Fatalln(err)
	}

	return
}

var pluralise = map[string]string{
	"Gateway":   "s",
	"Attribute": "s",
	"Type":      "s",
	"Include":   "s",
}

var defaults = map[string]string{
	"Includes": "100",
	"Gateways": "7039",
}

// XXX abstract this for a general case
func setMaps(c geneos.Instance) (err error) {
	if len(setCmdAttributes) > 0 {
		attr := c.V().GetStringMapString("attributes")
		for k, v := range setCmdAttributes {
			attr[k] = v
		}
		c.V().Set("attributes", attr)
	}

	if len(setCmdTypes) > 0 {
		types := c.V().GetStringSlice("types")
		for _, v := range setCmdTypes {
			types = append(types, v)
		}
		c.V().Set("types", types)
	}

	if len(setCmdEnvs) > 0 {
		envs := c.V().GetStringMapString("env")
		for k, v := range setCmdEnvs {
			envs[k] = v
		}
		c.V().Set("env", envs)
	}

	if len(setCmdGateways) > 0 {
		gateways := c.V().GetStringMapString("gateways")
		for k, v := range setCmdGateways {
			gateways[k] = v
		}
		c.V().Set("gateways", gateways)
	}

	if len(setCmdVariables) > 0 {
		vars := c.V().GetStringMapString("variables")
		for k, v := range setCmdVariables {
			vars[k] = v
		}
		c.V().Set("variables", vars)
	}

	return nil
}

func setValue(c geneos.Instance, tag, v string) (err error) {
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
			setStructMap(c, tag, e[0], "")
		} else {
			val := defaults[tag]
			if len(e) > 1 {
				val = e[1]
			} else {
				// XXX check two values and first is a number
				logDebug.Println("second value missing after ':', using default", val)
			}
			setStructMap(c, tag, e[0], val)
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
			setStructMap(c, tag, e[0], "")
		} else {
			setStructMap(c, tag, e[0], e[1])
		}
	case "Env", "Types":
		var remove bool
		slice := c.V().GetStringSlice(tag)
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
		c.V().Set(tag, newslice)
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
			setStructMap(c, tag, v, "")
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
		setStructMap(c, tag, key, val)
	default:
		c.V().Set(tag, v)
	}
	return
}

func setStructMap(c geneos.Instance, field, key, value string) {
	m := c.V().GetStringMapString(field)
	if value == "" {
		delete(m, key)
	} else {
		m[key] = value
	}
	c.V().Set(field, m)

}

// XXX muddled - fix
func writeConfigParams(filename string, params []string) (err error) {
	vp := viper.New()
	vp.SetConfigFile(filename)
	vp.ReadInConfig()

	// change here
	for _, set := range params {
		// skip all non '=' args
		if !strings.Contains(set, "=") {
			continue
		}
		s := strings.SplitN(set, "=", 2)
		k, v := s[0], s[1]
		vp.Set(k, v)
	}

	// fix breaking change
	if vp.IsSet("itrshome") {
		if !vp.IsSet("geneos") {
			vp.Set("geneos", vp.GetString("itrshome"))
		}
		vp.Set("itrshome", nil)
	}

	vp.WriteConfig()
	return nil
}

// Value types for multiple flags - also used by unset?

// include file - priority:url|path
type IncludeValues map[string]string

func (i *IncludeValues) String() string {
	return ""
}

func (i *IncludeValues) Set(value string) error {
	e := strings.SplitN(value, ":", 2)
	val := "100"
	if len(e) > 1 {
		val = e[1]
	} else {
		// XXX check two values and first is a number
		logDebug.Println("second value missing after ':', using default", val)
	}
	(*i)[e[0]] = val
	return nil
}

func (i *IncludeValues) Type() string {
	return "PRIORITY:{URL|PATH}"
}

// gateway - name:port
type GatewayValues map[string]string

func (i *GatewayValues) String() string {
	return ""
}

func (i *GatewayValues) Set(value string) error {
	e := strings.SplitN(value, ":", 2)
	val := "7039"
	if len(e) > 1 {
		val = e[1]
	} else {
		// XXX check two values and first is a number
		logDebug.Println("second value missing after ':', using default", val)
	}
	(*i)[e[0]] = val
	return nil
}

func (i *GatewayValues) Type() string {
	return "HOSTNAME:PORT"
}

// attribute - name=value
type NamedValues map[string]string

func (i *NamedValues) String() string {
	return ""
}

func (i *NamedValues) Set(value string) error {
	e := strings.SplitN(value, "=", 2)
	if len(e) < 2 {
		logError.Println("attributes must be in the format NAME=VALUE")
		return geneos.ErrInvalidArgs
	}
	(*i)[e[0]] = e[1]
	return nil
}

func (i *NamedValues) Type() string {
	return "NAME=VALUE"
}

// attribute - name=value
type TypeValues []string

func (i *TypeValues) String() string {
	return ""
}

func (i *TypeValues) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func (i *TypeValues) Type() string {
	return "NAME"
}

// variables - [TYPE:]NAME=VALUE
type VarValues map[string]string

func (i *VarValues) String() string {
	return ""
}

func (i *VarValues) Set(value string) error {
	var t, k, v string

	e := strings.SplitN(value, ":", 2)
	if len(e) == 1 {
		t = "string"
		s := strings.SplitN(e[0], "=", 2)
		k = s[0]
		if len(s) > 1 {
			v = s[1]
		}
	} else {
		t = e[0]
		s := strings.SplitN(e[1], "=", 2)
		k = s[0]
		if len(s) > 1 {
			v = s[1]
		}
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
	val := t + ":" + v
	(*i)[k] = val
	return nil
}

func (i *VarValues) Type() string {
	return "[TYPE:]NAME=VALUE"
}
