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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
)

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import [TYPE] [FLAGS | NAME [NAME...]] [DEST=]SOURCE [[DEST=]SOURCE...]",
	Short: "Import file(s) to an instance or a common directory",
	Long: `Import file(s) to the instance or common directory. This can be used
	to add configuration or license files or scripts for gateways and
	netprobes to run. The SOURCE can be a local path or a url or a '-'
	for stdin. DEST is local pathname ending in either a filename or a
	directory. Is the SRC is '-' then a DEST must be provided. If DEST
	includes a path then it must be relative and cannot contain '..'.
	Examples:
	
		geneos import gateway example1 https://example.com/myfiles/gateway.setup.xml
		geneos import licd example2 geneos.lic=license.txt
		geneos import netprobe example3 scripts/=myscript.sh
		geneos import san localhost ./netprobe.setup.xml
		geneos import gateway -c shared common_include.xml
	
	To distinguish SOURCE from an instance name a bare filename in the
	current directory MUST be prefixed with './'. A file in a directory
	(relative or absolute) or a URL are seen as invalid instance names
	and become paths automatically. Directories are created as required.
	If run as root, directories and files ownership is set to the user in
	the instance configuration or the default user. Currently only one
	file can be imported at a time.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("import called")
	},
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringVarP(&common, "common", "c", "", "Import into a common directory instead of matching instances.	If TYPE is 'gateway' and NAME is 'shared' then this common directory is 'gateway/gateway_shared'")
	importCmd.Flags().StringVarP(&hostname, "remote", "r", "all", "Import to named remote, default is all")
}

var common, hostname string

// add a file to an instance, from local or URL
// overwrites without asking - use case is license files, setup files etc.
// backup / history track older files (date/time?)
// no restart or reload of components?

func commandImport(ct *geneos.Component, args []string, params []string) (err error) {
	if common != "" {
		// ignore args, use ct & params
		rems := host.GetRemote(host.Name(hostname))
		if rems == host.ALL {
			for _, r := range host.AllHosts() {
				importCommons(r, ct, params)
			}
			return nil
		}
		return importCommons(rems, ct, params)
	}

	return instance.LoopCommand(ct, importInstance, args, params)
}

// args are instance [file...]
// file can be a local path or a url
// destination is basename of source in the home directory
// file can also be DEST=SOURCE where dest must be a relative path (with
// no ../) to home area, anding in / means subdir, e.g.:
//
// 'geneos import gateway example1 https://example.com/myfiles/gateway.setup.xml'
// 'geneos import licd example2 geneos.lic=license.txt'
// 'geneos import netprobe example3 scripts/=myscript.sh'
//
// local directories are created
func importInstance(c geneos.Instance, params []string) (err error) {
	if !c.Type().RealComponent {
		return ErrNotSupported
	}

	if len(params) == 0 {
		logError.Fatalln("no file/url provided")
	}

	for _, source := range params {
		if err = instance.ImportFile(c.Remote(), c.Home(), c.V().GetString(c.Prefix("User")), source); err != nil {
			return
		}
	}
	return
}

func importCommons(r *host.Host, ct *geneos.Component, params []string) (err error) {
	if !ct.RealComponent {
		return ErrNotSupported
	}

	if len(params) == 0 {
		logError.Fatalln("no file/url provided")
	}

	dir := r.GeneosPath(ct.String(), ct.String()+"_"+common)
	for _, source := range params {
		if err = instance.ImportFile(r, dir, viper.GetString("DefaultUser"), source); err != nil {
			return
		}
	}
	return
}
