package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func init() {
	commands["add"] = Command{
		Function:    commandAdd,
		ParseFlags:  defaultFlag,
		ParseArgs:   parseArgsNoWildcard,
		CommandLine: "geneos add TYPE NAME",
		Summary:     `Add a new instance`,
		Description: `Add a new instance called NAME with the TYPE supplied. The details will depends on the
TYPE. Currently the listening port is selected automatically and other options are defaulted. If
these need to be changed before starting, see the edit command.

Gateways are given a minimal configuration file.`}

	commands["upload"] = Command{
		Function:    commandUpload,
		ParseFlags:  defaultFlag,
		ParseArgs:   parseArgs,
		CommandLine: "geneos upload [TYPE] NAME [DEST=]SRC",
		Summary:     `Upload file(s) to an instance.`,
		Description: `Upload file(s) to the instance directory. This can be used to add configuration or license
files or scripts for gateways and netprobes to run. The SRC can be a local path or a url or a '-'
for stdin. DEST is local pathname ending in either a filename or a directory. Is the SRC is '-'
then a DEST must be provided. If DEST includes a path then it must be relative and cannot contain
'..'. Examples:

	geneos upload gateway example1 https://example.com/myfiles/gateway.setup.xml
	geneos upload licd example2 geneos.lic=license.txt
	geneos upload netprobe exampel3 scripts/=myscript.sh
	
Directories are created as required. If run as root, directories and files ownership is set to the
user in the instance configuration or the default user. Currently only one file can be uploaded at a
time.`}
}

// Add a single instance
//
// XXX argument validation is minimal
//
// remote support would be of the form name@remotename
//
func commandAdd(ct Component, args []string, params []string) (err error) {
	if len(args) == 0 {
		logError.Fatalln("not enough args")
	}

	// check validty and reserved words here
	name := args[0]

	var username string
	if superuser {
		username = GlobalConfig["DefaultUser"]
	} else {
		u, _ := user.Current()
		username = u.Username
	}

	// XXX check if instance already exists

	c := ct.New(name)
	if err = c.Create(username, params); err != nil {
		log.Fatalln(err)
	}
	// reload config as 'c' is not updated by Create() as an interface value
	loadConfig(c, false)
	log.Printf("new %s %q added, port %s\n", c.Type(), c.Name(), getIntAsString(c, c.Prefix("Port")))

	return
}

// get all used ports in config files.
// this will not work for ports assigned in component config
// files, such as gateway setup or netprobe collection agent
//
// returns a map
func getPorts(remote string) (ports map[int]Component) {
	ports = make(map[int]Component)
	for _, c := range getInstances(remote) {
		if err := loadConfig(c, false); err != nil {
			log.Println(c.Type(), c.Name(), "- cannot load configuration")
			continue
		}
		if port := getIntAsString(c, c.Prefix("Port")); port != "" {
			if p, err := strconv.Atoi(port); err == nil {
				ports[int(p)] = c.Type()
			}
		}
	}
	return
}

// syntax of ranges of ints:
// x,y,a-b,c..d m n o-p
// also open ended A,N-,B
// command or space seperated?
// - or .. = inclusive range
//
// how to represent
// split, for range, check min-max -> max > min
// repeats ignored
// special ports? - nah
//

// given a range, find the first unsed port
//
// range is comma or two-dot seperated list of
// single number, e.g. "7036"
// min-max inclusive range, e.g. "7036-8036"
// start- open ended range, e.g. "7041-"
//
// some limits based on https://en.wikipedia.org/wiki/List_of_TCP_and_UDP_port_numbers
//
// not concurrency safe at this time
//
func nextPort(remote string, from string) int {
	used := getPorts(remote)
	ps := strings.Split(from, ",")
	for _, p := range ps {
		// split on comma or ".."
		m := strings.SplitN(p, "-", 2)
		if len(m) == 1 {
			m = strings.SplitN(p, "..", 2)
		}

		if len(m) > 1 {
			min, err := strconv.Atoi(m[0])
			if err != nil {
				continue
			}
			if m[1] == "" {
				m[1] = "49151"
			}
			max, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			if min >= max {
				continue
			}
			for i := min; i <= max; i++ {
				if _, ok := used[i]; !ok {
					// found an unused port
					return i
				}
			}
		} else {
			p1, err := strconv.Atoi(m[0])
			if err != nil || p1 < 1 || p1 > 49151 {
				continue
			}
			if _, ok := used[p1]; !ok {
				return p1
			}
		}
	}
	return 0
}

// add a file to an instance, from local or URL
// overwrites without asking - use case is license files, setup files etc.
// backup / history track older files (date/time?)
// no restart or reload of compnents?

func commandUpload(ct Component, args []string, params []string) (err error) {
	return ct.singleCommand(uploadInstance, args, params)
}

// args are instance [file...]
// file can be a local path or a url
// destination is basename of source in the home directory
// file can also be DEST=SOURCE where dest must be a relative path (with
// no ../) to home area, anding in / means subdir, e.g.:
//
// 'geneos upload gateway example1 https://example.com/myfiles/gateway.setup.xml'
// 'geneos upload licd example2 geneos.lic=license.txt'
// 'geneos upload netprobe exampel3 scripts/=myscript.sh'
//
// local directroreies are created
func uploadInstance(c Instances, args []string, params []string) (err error) {
	if !components[c.Type()].IncludeInLoops {
		return ErrNotSupported
	}

	if len(params) == 0 {
		logError.Fatalln("no file/url provided")
	}

	for _, source := range params {
		if err = uploadFile(c, source); err != nil {
			return
		}
	}
	return
}

func uploadFile(c Instances, source string) (err error) {
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
			if s, err := statFile(c.Location(), filepath.Join(c.Home(), destfile)); err == nil {
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
		from, _, err = statAndOpenFile(LOCAL, source)
		if err != nil {
			return err
		}
		if destfile == "" {
			destfile = filepath.Base(source)
		}
		defer from.Close()
	}

	destfile = filepath.Join(destdir, destfile)

	if _, err := statFile(c.Location(), filepath.Dir(destfile)); err != nil {
		err = mkdirAll(c.Location(), filepath.Dir(destfile), 0775)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			logError.Fatalln(err)
		}
		// if created, chown the last element
		if err == nil {
			if err = chown(c.Location(), filepath.Dir(destfile), int(uid), int(gid)); err != nil {
				return err
			}
		}
	}

	// xxx - wrong way around. create tmp first, move over later
	if s, err := statFile(c.Location(), destfile); err == nil {
		if !s.st.Mode().IsRegular() {
			logError.Fatalln("dest exists and is not a plain file")
		}
		datetime := time.Now().UTC().Format("20060102150405")
		backuppath = destfile + "." + datetime + ".old"
		if err = renameFile(c.Location(), destfile, backuppath); err != nil {
			return err
		}
	}

	var out io.Writer

	switch c.Location() {
	case LOCAL:
		cf, err := os.Create(destfile)
		if err != nil {
			return err
		}
		out = cf
		defer cf.Close()

		if err = cf.Chown(int(uid), int(gid)); err != nil {
			removeFile(c.Location(), destfile)
			if backuppath != "" {
				if err = renameFile(c.Location(), backuppath, destfile); err != nil {
					return err
				}
				return err
			}
		}
	default:
		cf, err := createRemoteFile(c.Location(), destfile)
		if err != nil {
			return err
		}
		out = cf
		defer cf.Close()

		if err = cf.Chown(int(uid), int(gid)); err != nil {
			removeFile(c.Location(), destfile)
			if backuppath != "" {
				if err = renameFile(c.Location(), backuppath, destfile); err != nil {
					return err
				}
				return err
			}
		}
	}

	if _, err = io.Copy(out, from); err != nil {
		return err
	}
	log.Println("uploaded", source, "to", destfile)
	return nil
}
