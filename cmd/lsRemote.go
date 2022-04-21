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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	geneos "wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/instance"
)

// lsRemoteCmd represents the lsRemote command
var lsRemoteCmd = &cobra.Command{
	Use:   "remote [-c|-j [-i]] [TYPE] [NAME...]",
	Short: "List remotes, optionally in CSV or JSON format",
	Long:  `List the matching remotes.`,
	Annotations: map[string]string{
		"wildcard": "true",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := processArgs(cmd)
		return commandLSRemote(ct, args, params)
	},
}

func init() {
	lsCmd.AddCommand(lsRemoteCmd)

	lsRemoteCmd.PersistentFlags().BoolVarP(&lsRemoteCmdJSON, "json", "j", false, "Output JSON")
	lsRemoteCmd.PersistentFlags().BoolVarP(&lsRemoteCmdIndent, "indent", "i", false, "Indent / pretty print JSON")
	lsRemoteCmd.PersistentFlags().BoolVarP(&lsRemoteCmdCSV, "csv", "c", false, "Output CSV")
}

var lsRemoteCmdJSON, lsRemoteCmdCSV, lsRemoteCmdIndent bool

func commandLSRemote(ct *geneos.Component, args []string, params []string) (err error) {
	switch {
	case lsRemoteCmdJSON:
		jsonEncoder = json.NewEncoder(log.Writer())
		if lsRemoteCmdIndent {
			jsonEncoder.SetIndent("", "    ")
		}
		err = instance.LoopCommand(ct, lsInstanceJSONRemotes, args, params)
	case lsRemoteCmdCSV:
		csvWriter = csv.NewWriter(log.Writer())
		csvWriter.Write([]string{"Type", "Name", "Disabled", "Username", "Hostname", "Port", "Geneos"})
		err = instance.LoopCommand(ct, lsInstanceCSVRemotes, args, params)
		csvWriter.Flush()
	default:
		lsTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
		fmt.Fprintf(lsTabWriter, "Type\tName\tUsername\tHostname\tPort\tITRSHome\n")
		err = instance.LoopCommand(ct, lsInstancePlainRemotes, args, params)
		lsTabWriter.Flush()
	}
	if err == os.ErrNotExist {
		err = nil
	}
	return
}

func lsInstancePlainRemotes(c geneos.Instance, params []string) (err error) {
	var suffix string
	if instance.IsDisabled(c) {
		suffix = "*"
	}
	fmt.Fprintf(lsTabWriter, "%s\t%s\t%s\t%s\t%d\t%s\n", c.Type(), c.Name()+suffix, c.V().GetString("Username"), c.V().GetString("Hostname"), c.V().GetInt("Port"), c.V().GetString("Geneos"))
	return
}

func lsInstanceCSVRemotes(c geneos.Instance, params []string) (err error) {
	var dis string = "N"
	if instance.IsDisabled(c) {
		dis = "Y"
	}
	csvWriter.Write([]string{c.Type().String(), c.Name(), dis, c.V().GetString("Username"), c.V().GetString("Hostname"), fmt.Sprint(c.V().GetInt("Port")), c.V().GetString("Geneos")})
	return
}

type lsTypeRemotes struct {
	Type     string
	Name     string
	Disabled string
	Username string
	Hostname string
	Port     int64
	Geneos   string
}

func lsInstanceJSONRemotes(c geneos.Instance, params []string) (err error) {
	var dis string = "N"
	if instance.IsDisabled(c) {
		dis = "Y"
	}
	jsonEncoder.Encode(lsTypeRemotes{c.Type().String(), c.Name(), dis, c.V().GetString("Username"), c.V().GetString("Hostname"), c.V().GetInt64("Port"), c.V().GetString("Geneos")})
	return
}
