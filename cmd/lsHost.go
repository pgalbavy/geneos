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
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
)

// lsHostCmd represents the lsRemote command
var lsHostCmd = &cobra.Command{
	Use:                   "host [-c|-j [-i]] [TYPE] [NAME...]",
	Aliases:               []string{"hosts", "remote", "remotes"},
	Short:                 "List hosts, optionally in CSV or JSON format",
	Long:                  `List the matching remote hosts.`,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "false",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := cmdArgsParams(cmd)
		return commandLSHost(ct, args, params)
	},
}

func init() {
	lsCmd.AddCommand(lsHostCmd)

	lsHostCmd.PersistentFlags().BoolVarP(&lsHostCmdJSON, "json", "j", false, "Output JSON")
	lsHostCmd.PersistentFlags().BoolVarP(&lsHostCmdIndent, "pretty", "i", false, "Indent / pretty print JSON")
	lsHostCmd.PersistentFlags().BoolVarP(&lsHostCmdCSV, "csv", "c", false, "Output CSV")
	lsHostCmd.Flags().SortFlags = false
}

var lsHostCmdJSON, lsHostCmdCSV, lsHostCmdIndent bool

func commandLSHost(ct *geneos.Component, args []string, params []string) (err error) {
	switch {
	case lsHostCmdJSON:
		jsonEncoder = json.NewEncoder(log.Writer())
		if lsHostCmdIndent {
			jsonEncoder.SetIndent("", "    ")
		}
		err = loopHosts(lsInstanceJSONHosts)
	case lsHostCmdCSV:
		csvWriter = csv.NewWriter(log.Writer())
		csvWriter.Write([]string{"Type", "Name", "Disabled", "Username", "Hostname", "Port", "Directory"})
		err = loopHosts(lsInstanceCSVHosts)
		csvWriter.Flush()
	default:
		lsTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
		fmt.Fprintf(lsTabWriter, "Name\tUsername\tHostname\tPort\tDirectory\n")
		err = loopHosts(lsInstancePlainHosts)
		lsTabWriter.Flush()
	}
	if err == os.ErrNotExist {
		err = nil
	}
	return
}

func loopHosts(fn func(*host.Host) error) error {
	for _, h := range host.AllHosts() {
		if h == host.LOCAL {
			continue
		}
		fn(h)
	}
	return nil
}

func lsInstancePlainHosts(h *host.Host) (err error) {
	fmt.Fprintf(lsTabWriter, "%s\t%s\t%s\t%d\t%s\n", h.GetString("name"), h.GetString("username"), h.GetString("hostname"), h.GetInt("port"), h.GetString("geneos"))
	return
}

func lsInstanceCSVHosts(h *host.Host) (err error) {
	csvWriter.Write([]string{h.String(), h.GetString("username"), h.GetString("hostname"), fmt.Sprint(h.GetInt("port")), h.GetString("geneos")})
	return
}

type lsTypeHosts struct {
	Name      string
	Username  string
	Hostname  string
	Port      int64
	Directory string
}

func lsInstanceJSONHosts(h *host.Host) (err error) {
	jsonEncoder.Encode(lsTypeHosts{h.String(), h.GetString("username"), h.GetString("hostname"), h.GetInt64("port"), h.GetString("geneos")})
	return
}
