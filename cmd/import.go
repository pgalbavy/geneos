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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/component"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/internal/utils"
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
// no restart or reload of compnents?

func commandImport(ct component.ComponentType, args []string, params []string) (err error) {
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
// local directroreies are created
func importInstance(c instance.Instance, params []string) (err error) {
	if !components[c.Type()].RealComponent {
		return ErrNotSupported
	}

	if len(params) == 0 {
		logError.Fatalln("no file/url provided")
	}

	for _, source := range params {
		if err = importFile(c.Remote(), c.Home(), c.V.GetString(c.Prefix("User")), source); err != nil {
			return
		}
	}
	return
}

func importCommons(r *host.Host, ct component.ComponentType, params []string) (err error) {
	if !components[ct].RealComponent {
		return ErrNotSupported
	}

	if len(params) == 0 {
		logError.Fatalln("no file/url provided")
	}

	dir := r.GeneosPath(ct.String(), ct.String()+"_"+common)
	for _, source := range params {
		if err = importFile(r, dir, viper.GetString("DefaultUser", source); err != nil {
			return
		}
	}
	return
}

// only use of Instances is for remote and home

// func importFile(c Instances, source string) (err error) {
func importFile(r *host.Host, home string, user string, source string) (err error) {
	var backuppath string
	var from io.ReadCloser

	if r == host.ALL {
		return ErrInvalidArgs
	}

	uid, gid, _, err := utils.GetUser(user)
	if err != nil {
		return err
	}

	destdir := home
	destfile := ""

	// if the source is a http(s) url then skip '=' split (protect queries in URL)
	if !strings.HasPrefix(source, "https://") && !strings.HasPrefix(source, "http://") {
		splitsource := strings.SplitN(source, "=", 2)
		if len(splitsource) > 1 {
			// do some basic validation on user-supplied destination
			if splitsource[0] == "" {
				logError.Fatalln("dest path empty")
			}
			destfile, err = host.CleanRelativePath(splitsource[0])
			if err != nil {
				logError.Fatalln("dest path must be relative to (and in) instance directory")
			}
			// if the destination exists is it a directory?
			if s, err := r.Stat(filepath.Join(home, destfile)); err == nil {
				if s.St.IsDir() {
					destdir = filepath.Join(home, destfile)
					destfile = ""
				}
			}
			source = splitsource[1]
			if source == "" {
				logError.Fatalln("no source defined")
			}
		}
	}

	// see if it's a URL
	u, err := url.Parse(source)
	if err != nil {
		return err
	}

	switch {
	case u.Scheme == "https" || u.Scheme == "http":
		resp, err := http.Get(u.String())
		if err != nil {
			return err
		}
		if resp.StatusCode > 299 {
			err = fmt.Errorf("cannot download %q: %s", source, resp.Status)
			resp.Body.Close()
			return err
		}

		if destfile == "" {
			// XXX check content-disposition or use basename or response URL if no destfile defined
			destfile, err = component.FilenameFromHTTPResp(resp, u)
			if err != nil {
				logError.Fatalln(err)
			}
		}

		from = resp.Body
		defer from.Close()

	case source == "-":
		if destfile == "" {
			logError.Fatalln("for stdin a destination file must be provided, e.g. file.txt=-")
		}
		from = os.Stdin
		source = "STDIN"
		defer from.Close()

	default:
		// support globbing later
		from, err = host.LOCAL.Open(source)
		if err != nil {
			return err
		}
		if destfile == "" {
			destfile = filepath.Base(source)
		}
		defer from.Close()
	}

	destfile = filepath.Join(destdir, destfile)

	if _, err := r.Stat(filepath.Dir(destfile)); err != nil {
		err = r.MkdirAll(filepath.Dir(destfile), 0775)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			logError.Fatalln(err)
		}
		// if created, chown the last element
		if err == nil {
			if err = r.Chown(filepath.Dir(destfile), uid, gid); err != nil {
				return err
			}
		}
	}

	// xxx - wrong way around. create tmp first, move over later
	if s, err := r.Stat(destfile); err == nil {
		if !s.St.Mode().IsRegular() {
			logError.Fatalln("dest exists and is not a plain file")
		}
		datetime := time.Now().UTC().Format("20060102150405")
		backuppath = destfile + "." + datetime + ".old"
		if err = r.Rename(destfile, backuppath); err != nil {
			return err
		}
	}

	cf, err := r.Create(destfile, 0664)
	if err != nil {
		return err
	}
	defer cf.Close()

	if err = r.Chown(destfile, uid, gid); err != nil {
		r.Remove(destfile)
		if backuppath != "" {
			if err = r.Rename(backuppath, destfile); err != nil {
				return err
			}
			return err
		}
	}

	if _, err = io.Copy(cf, from); err != nil {
		return err
	}
	log.Printf("imported %q to %s:%s", source, r.String(), destfile)
	return nil
}
