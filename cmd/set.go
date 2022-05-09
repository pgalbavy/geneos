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
	Use:   "set [FLAGS] [TYPE] [NAME...] KEY=VALUE [KEY=VALUE...]",
	Short: "Set instance configuration parameters",
	Long: `Set configuration item values in global, user, or for a specific
instance.

To set "special" items, such as Environment variables or Attributes you should
now use the specific flags and not the old special syntax.

The "set" command does not rebuild any configuration files for instances.
Use "rebuild" to do this.`,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := cmdArgsParams(cmd)
		return commandSet(ct, args, params)
	},
}

func init() {
	rootCmd.AddCommand(setCmd)

	setCmd.Flags().VarP(&setCmdExtras.Envs, "env", "e", "(all components) Add an environment variable in the format NAME=VALUE")
	setCmd.Flags().VarP(&setCmdExtras.Includes, "include", "i", "(gateways) Add an include file in the format PRIORITY:PATH")
	setCmd.Flags().VarP(&setCmdExtras.Gateways, "gateway", "g", "(sans) Add a gateway in the format NAME:PORT")
	setCmd.Flags().VarP(&setCmdExtras.Attributes, "attribute", "a", "(sans) Add an attribute in the format NAME=VALUE")
	setCmd.Flags().VarP(&setCmdExtras.Types, "type", "t", "(sans) Add a type NAME")
	setCmd.Flags().VarP(&setCmdExtras.Variables, "variable", "v", "(sans) Add a variable in the format [TYPE:]NAME=VALUE")
	setCmd.Flags().SortFlags = false
}

var setCmdExtras = instance.ExtraConfigValues{
	Includes:   instance.IncludeValues{},
	Gateways:   instance.GatewayValues{},
	Attributes: instance.StringSliceValues{},
	Envs:       instance.StringSliceValues{},
	Variables:  instance.VarValues{},
	Types:      instance.StringSliceValues{},
}

func commandSet(ct *geneos.Component, args, params []string) error {
	return instance.ForAll(ct, setInstance, args, params)
}

func setInstance(c geneos.Instance, params []string) (err error) {
	logDebug.Println("c", c, "params", params)

	// walk through any flags passed
	instance.SetExtendedValues(c, setCmdExtras)

	for _, arg := range params {
		s := strings.SplitN(arg, "=", 2)
		if len(s) != 2 {
			logError.Printf("ignoring %q %s", arg, ErrInvalidArgs)
			continue
		}
		c.V().Set(s[0], s[1])
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

	return vp.WriteConfig()
}

func readConfigFile(path string) (v *viper.Viper) {
	v = viper.New()
	v.SetConfigFile(path)
	v.ReadInConfig()
	return
}
