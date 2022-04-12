package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	RegisterCommand(Command{
		Name:          "import",
		Function:      commandImport,
		ParseFlags:    importFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos import [TYPE] [FLAGS | NAME [NAME...]] [DEST=]SOURCE [[DEST=]SOURCE...]",
		Summary:       `Import file(s) to an instance or a common directory.`,
		Description: `Import file(s) to the instance or common directory. This can be used
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
file can be imported at a time.

FLAGS:
	-c NAME		import into a common directory instead of matching instances.
			If TYPE is 'gateway' and NAME is 'shared' then this common directory
			is 'gateway/gateway_shared'
	-r REMOTE	Limits the 'common' import to a specific remote. Default is 'all' 
			For instance imports user the 'instance@remote' or '@remote' syntax
`})

	importFlags = flag.NewFlagSet("import", flag.ExitOnError)
	importFlags.StringVar(&importCommon, "c", "", "Import into a common directory named xxx_COMMON")
	importFlags.StringVar(&importRemote, "r", "all", "Import to named remote, default is all")
}

var importFlags *flag.FlagSet
var importCommon string
var importRemote string

func importFlag(command string, args []string) []string {
	importFlags.Parse(args)
	checkHelpFlag(command)
	return importFlags.Args()
}

// add a file to an instance, from local or URL
// overwrites without asking - use case is license files, setup files etc.
// backup / history track older files (date/time?)
// no restart or reload of compnents?

func commandImport(ct Component, args []string, params []string) (err error) {
	if importCommon != "" {
		// ignore args, use ct & params
		rems := GetRemote(RemoteName(importRemote))
		if rems == rALL {
			for _, r := range AllRemotes() {
				importCommons(ct, r, params)
			}
			return nil
		}
		return importCommons(ct, rems, params)
	}

	return ct.loopCommand(importInstance, args, params)
}

// args are instance [file...]
// file can be a local path or a url
// destination is basename of source in the home directory
// file can also be DEST=SOURCE where dest must be a relative path (with
// no ../) to home area, anding in / means subdir, e.g.:
//
// 'geneos import gateway example1 https://example.com/myfiles/gateway.setup.xml'
// 'geneos import licd example2 geneos.lic=license.txt'
// 'geneos import netprobe exampel3 scripts/=myscript.sh'
//
// local directroreies are created
func importInstance(c Instance, params []string) (err error) {
	if !components[c.Type()].RealComponent {
		return ErrNotSupported
	}

	if len(params) == 0 {
		logError.Fatalln("no file/url provided")
	}

	for _, source := range params {
		if err = importFile(c.Remote(), c.Home(), getString(c, c.Prefix("User")), source); err != nil {
			return
		}
	}
	return
}

func importCommons(ct Component, r *Remotes, params []string) (err error) {
	if !components[ct].RealComponent {
		return ErrNotSupported
	}

	if len(params) == 0 {
		logError.Fatalln("no file/url provided")
	}

	dir := r.GeneosPath(ct.String(), ct.String()+"_"+importCommon)
	for _, source := range params {
		if err = importFile(r, dir, GlobalConfig["DefaultUser"], source); err != nil {
			return
		}
	}
	return
}

// only use of Instances is for remote and home

// func importFile(c Instances, source string) (err error) {
func importFile(r *Remotes, home string, user string, source string) (err error) {
	var backuppath string
	var from io.ReadCloser

	if r == rALL {
		return ErrInvalidArgs
	}

	uid, gid, _, err := getUser(user)
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
			destfile, err = cleanRelativePath(splitsource[0])
			if err != nil {
				logError.Fatalln("dest path must be relative to (and in) instance directory")
			}
			// if the destination exists is it a directory?
			if s, err := r.Stat(filepath.Join(home, destfile)); err == nil {
				if s.st.IsDir() {
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
			destfile, err = filenameFromHTTPResp(resp, u)
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
		from, err = rLOCAL.Open(source)
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
		if !s.st.Mode().IsRegular() {
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
	log.Printf("imported %q to %s:%s", source, r.InstanceName, destfile)
	return nil
}
