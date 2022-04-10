package main

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
)

func init() {
	RegsiterCommand(Command{
		Name:          "import",
		Function:      commandImport,
		ParseFlags:    defaultFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos import [TYPE] NAME [NAME...] [DEST=]SOURCE [[DEST=]SOURCE...]",
		Summary:       `Import file(s) to an instance.`,
		Description: `Import file(s) to the instance directory. This can be used to add
configuration or license files or scripts for gateways and netprobes
to run. The SOURCE can be a local path or a url or a '-' for stdin. DEST
is local pathname ending in either a filename or a directory. Is the
SRC is '-' then a DEST must be provided. If DEST includes a path then
it must be relative and cannot contain '..'. Examples:

	geneos import gateway example1 https://example.com/myfiles/gateway.setup.xml
	geneos import licd example2 geneos.lic=license.txt
	geneos import netprobe example3 scripts/=myscript.sh
	geneos import san localhost ./netprobe.setup.xml

To distinguish SOURCE from an instance name a bare filename in the
current directory MUST be prefixed with './'. A file in a directory
(relative or absolute) or a URL are seen as invalid instance names
and become paths automatically. Directories are created as required.
If run as root, directories and files ownership is set to the user in
the instance configuration or the default user. Currently only one
file can be imported at a time.`})
}

// add a file to an instance, from local or URL
// overwrites without asking - use case is license files, setup files etc.
// backup / history track older files (date/time?)
// no restart or reload of compnents?

func commandImport(ct Component, args []string, params []string) (err error) {
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
func importInstance(c Instances, params []string) (err error) {
	if !components[c.Type()].RealComponent {
		return ErrNotSupported
	}

	if len(params) == 0 {
		logError.Fatalln("no file/url provided")
	}

	for _, source := range params {
		if err = importFile(c, source); err != nil {
			return
		}
	}
	return
}

func importFile(c Instances, source string) (err error) {
	var backuppath string
	var from io.ReadCloser

	uid, gid, _, err := getUser(getString(c, c.Prefix("User")))
	if err != nil {
		return err
	}

	destdir := c.Home()
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
			if s, err := c.Remote().Stat(filepath.Join(c.Home(), destfile)); err == nil {
				if s.st.IsDir() {
					destdir = filepath.Join(c.Home(), destfile)
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

	if _, err := c.Remote().Stat(filepath.Dir(destfile)); err != nil {
		err = c.Remote().MkdirAll(filepath.Dir(destfile), 0775)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			logError.Fatalln(err)
		}
		// if created, chown the last element
		if err == nil {
			if err = c.Remote().Chown(filepath.Dir(destfile), uid, gid); err != nil {
				return err
			}
		}
	}

	// xxx - wrong way around. create tmp first, move over later
	if s, err := c.Remote().Stat(destfile); err == nil {
		if !s.st.Mode().IsRegular() {
			logError.Fatalln("dest exists and is not a plain file")
		}
		datetime := time.Now().UTC().Format("20060102150405")
		backuppath = destfile + "." + datetime + ".old"
		if err = c.Remote().Rename(destfile, backuppath); err != nil {
			return err
		}
	}

	cf, err := c.Remote().Create(destfile, 0664)
	if err != nil {
		return err
	}
	defer cf.Close()

	if err = c.Remote().Chown(destfile, uid, gid); err != nil {
		c.Remote().Remove(destfile)
		if backuppath != "" {
			if err = c.Remote().Rename(backuppath, destfile); err != nil {
				return err
			}
			return err
		}
	}

	if _, err = io.Copy(cf, from); err != nil {
		return err
	}
	log.Println("imported", source, "to", destfile)
	return nil
}
