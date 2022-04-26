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
	"encoding/json"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/instance"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show runtime, global, user or instance configuration is JSON format",
	Long: `Show the runtime, global, user or instance configuration.

	With no arguments show the resolved runtime configuration that
	results from environment variables, loading built-in defaults and the
	global and user configurations.
	
	If the sub-command 'global' or 'user' is supplied then any
	on-disk configuration for the respective options will be shown.
	
	If a component TYPE and/or instance NAME(s) are supplied then the
	configuration for those instances are output as JSON. This is
	regardless of the instance using a legacy .rc file or a native JSON
	configuration.
	
	Passwords and secrets are redacted in a very simplistic manner simply
	to prevent visibility in casual viewing.`,
	SilenceUsage: true,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := processArgs(cmd)
		return commandShow(ct, args, params)
	},
}

func init() {
	rootCmd.AddCommand(showCmd)

	// showCmd.Flags().BoolVarP(&showCmdYAML, "yaml", "y", false, "Show as YAML")
}

// var showCmdYAML bool

func commandShow(ct *geneos.Component, args []string, params []string) (err error) {
	return instance.ForAll(ct, showInstance, args, params)
}

type config struct {
	Name   string      `json:"name,omitempty"`
	Host   string      `json:"host,omitempty"`
	Config interface{} `json:"config,omitempty"`
}

func showInstance(c geneos.Instance, params []string) (err error) {
	var buffer []byte

	// remove aliases
	nv := viper.New()
	for _, k := range c.V().AllKeys() {
		if _, ok := c.Type().Aliases[k]; !ok {
			nv.Set(k, c.V().Get(k))
		}
	}

	// XXX wrap in location and type
	cf := &config{Name: c.Name(), Host: c.Host().String(), Config: nv.AllSettings()}

	// if showCmdYAML {
	// 	buffer, err = yaml.Marshal(cf)
	// } else {

	if buffer, err = json.MarshalIndent(cf, "", "    "); err != nil {
		return
	}
	buffer = opaqueJSONSecrets(buffer)
	// }
	log.Printf("%s\n", string(buffer))

	return
}

// XXX redact passwords - any field matching some regexp ?
// also embedded Envs
//
//
var red1 = regexp.MustCompile(`"(.*((?i)pass|password|secret))": "(.*)"`)
var red2 = regexp.MustCompile(`"(.*((?i)pass|password|secret))=(.*)"`)

func opaqueJSONSecrets(j []byte) []byte {
	// simple redact - and left field with "Pass" in it gets the right replaced
	j = red1.ReplaceAll(j, []byte(`"$1": "********"`))
	j = red2.ReplaceAll(j, []byte(`"$1=********"`))
	return j
}
