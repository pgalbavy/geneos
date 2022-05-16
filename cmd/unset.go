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
	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/instance"
)

// unsetCmd represents the unset command
var unsetCmd = &cobra.Command{
	Use:   "unset [FLAGS] [TYPE] [NAME...]",
	Short: "Unset a configuration value",
	Long: `Unset a configuration value.
	
This command has been added to remove the confusing negation syntax in set`,
	Example: `
geneos unset gateway GW1 -k aesfile
geneos unset san -g Gateway1
`,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args := cmdArgs(cmd)
		return commandUnset(ct, args)
	},
}

func init() {
	rootCmd.AddCommand(unsetCmd)
	unsetCmd.Flags().VarP(&unsetCmdKeys, "key", "k", "Unset a configuration key item")
	unsetCmd.Flags().VarP(&unsetCmdEnvs, "env", "e", "Remove an environment variable of NAME")
	unsetCmd.Flags().VarP(&unsetCmdIncludes, "include", "i", "Remove an include file in the format PRIORITY")
	unsetCmd.Flags().VarP(&unsetCmdGateways, "gateway", "g", "Remove gateway NAME")
	unsetCmd.Flags().VarP(&unsetCmdAttributes, "attribute", "a", "Remove an attribute of NAME")
	unsetCmd.Flags().VarP(&unsetCmdTypes, "type", "t", "Remove the type NAME")
	unsetCmd.Flags().VarP(&unsetCmdVariables, "variable", "v", "Remove a variable of NAME")
	unsetCmd.Flags().SortFlags = false
}

var unsetCmdKeys = unsetCmdValues{}
var unsetCmdIncludes = unsetCmdValues{}
var unsetCmdGateways = unsetCmdValues{}
var unsetCmdAttributes = unsetCmdValues{}
var unsetCmdEnvs = unsetCmdValues{}
var unsetCmdVariables = unsetCmdValues{}
var unsetCmdTypes = unsetCmdValues{}

func commandUnset(ct *geneos.Component, args []string) error {
	return instance.ForAll(ct, unsetInstance, args, []string{})
}

func unsetInstance(c geneos.Instance, params []string) (err error) {
	var changed bool
	logDebug.Println("c", c, "params", params)

	changed, err = unsetMaps(c)

	s := c.V().AllSettings()

	if len(unsetCmdKeys) > 0 {
		for _, k := range unsetCmdKeys {
			delete(s, k)
			changed = true
		}
	}
	if changed {
		if err = instance.Migrate(c); err != nil {
			logError.Fatalln("cannot migrate existing .rc config to set values in new .json configration file:", err)
		}

		if err = instance.WriteConfigValues(c, s); err != nil {
			logError.Fatalln(err)
		}
	}

	return
}

// XXX abstract this for a general case
func unsetMaps(c geneos.Instance) (changed bool, err error) {
	if unset(c, unsetCmdAttributes, "attributes") {
		changed = true
	}

	if unset(c, unsetCmdEnvs, "env") {
		changed = true
	}

	if unset(c, unsetCmdGateways, "gateways") {
		changed = true
	}

	if unset(c, unsetCmdIncludes, "includes") {
		changed = true
	}

	if unset(c, unsetCmdVariables, "variables") {
		changed = true
	}

	if len(unsetCmdTypes) > 0 {
		newtypes := []string{}
		types := c.V().GetStringSlice("types")
	OUTER:
		for _, t := range types {
			for _, v := range unsetCmdTypes {
				if t == v {
					changed = true
					continue OUTER
				}
			}
			newtypes = append(newtypes, t)
		}
		c.V().Set("types", newtypes)
	}

	return
}

func unset(c geneos.Instance, items unsetCmdValues, key string) (changed bool) {
	x := c.V().GetStringMapString(key)
	for _, k := range items {
		delete(x, k)
		changed = true
	}
	c.V().Set(key, x)
	return
}

// unset Var flags take just the key, either a name or a priority for include files
type unsetCmdValues []string

func (i *unsetCmdValues) String() string {
	return ""
}

func (i *unsetCmdValues) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func (i *unsetCmdValues) Type() string {
	return "SETTING"
}
