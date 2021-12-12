package main

import (
	"errors"
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
	commands["create"] = Command{commandCreate, parseArgs, "create", ""}
	commands["upload"] = Command{commandUpload, parseArgs, "upload a file", ""}
}

// call the component specific create functions
//
// like "init", diff if root or not
func commandCreate(ct ComponentType, args []string) (err error) {
	if len(args) == 0 {
		log.Fatalln("not enough args")
	}

	// check validty and reserved words here
	name := args[0]

	var username string
	if superuser {
		username = RunningConfig.DefaultUser
	} else {
		u, _ := user.Current()
		username = u.Username
	}

	switch ct {
	case None, Unknown:
		log.Fatalln("component type must be specified")
	case Gateways:
		gatewayCreate(name, username)
	case Netprobes:
		netprobeCreate(name, username)
	default:

		return ErrNotSupported
	}
	return
}

// get all used ports in config files.
// this will not work for ports assigned in component config
// files, such as gateway setup or netprobe collection agent
//
// returns a map
func getPorts() (ports map[int]ComponentType) {
	ports = make(map[int]ComponentType)
	for _, c := range allInstances() {
		if err := loadConfig(&c, false); err != nil {
			log.Println(Type(c), Name(c), "- cannot load configuration")
			continue
		}
		if port := getIntAsString(c, Prefix(c)+"Port"); port != "" {
			if p, err := strconv.Atoi(port); err == nil {
				ports[int(p)] = Type(c)
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
func nextPort(from string) int {
	used := getPorts()
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

func commandUpload(ct ComponentType, args []string) (err error) {
	return singleCommand(uploadInstance, ct, args)
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
//
// account for superuser stuff
//
func uploadInstance(c Instance, args []string) (err error) {
	var destfile, backuppath string
	var destdir bool
	var from io.ReadCloser

	if len(args) == 0 {
		log.Fatalln("no file name provided")
	}

	uid, gid, _, err := getUser(getString(c, Prefix(c)+"User"))
	if err != nil {
		return err
	}

	switch Type(c) {
	case Gateways:
		source := args[0]
		s := strings.SplitN(source, "=", 2)
		// did we have an '=' ?
		if len(s) == 2 {
			// do some basic validation on user-supplied destination
			destfile = s[0]
			if destfile == "" {
				log.Fatalln("dest path empty")
			}
			p := strings.Split(destfile, string(filepath.Separator))
			if len(p) > 0 && len(p[0]) == 0 {
				log.Fatalln("dest path must be relative")
			}
			for _, e := range p {
				if e == ".." {
					log.Fatalln("dest path cannot contain '..'")
				}
			}
			destfile = filepath.Clean(destfile)
			if st, err := os.Stat(filepath.Join(Home(c), destfile)); err == nil {
				if st.IsDir() {
					destdir = true
				}
			}
			source = s[1]
			if source == "" {
				log.Fatalln("no source defined")
			}
		}
		u, err := url.Parse(source)
		if err != nil {
			return err
		}
		if strings.HasPrefix(u.Scheme, "http") {
			resp, err := http.Get(u.String())
			if err != nil {
				return err
			}
			from = resp.Body
			if destdir {
				destfile = filepath.Join(destfile, filepath.Base(resp.Request.URL.Path))
			} else if destfile == "" {
				destfile = filepath.Base(resp.Request.URL.Path)
			}
			defer from.Close()
		} else if source == "-" {
			if destfile == "" || destdir {
				log.Fatalln("for stdin a destination file must be provided, e.g. file.txt=-")
			}
			from = os.Stdin
		} else {
			// support globbing later
			from, err = os.Open(source)
			if err != nil {
				return err
			}
			if destdir {
				destfile = filepath.Join(destfile, filepath.Base(source))
			} else if destfile == "" {
				destfile = filepath.Base(source)
			}
			defer from.Close()
		}

		destpath := filepath.Join(Home(c), destfile)
		if _, err := os.Stat(filepath.Dir(destpath)); err != nil {
			err = os.MkdirAll(filepath.Dir(destpath), 0775)
			if err != nil && !errors.Is(err, fs.ErrExist) {
				log.Fatalln(err)
			}
			// if created, chown the last element
			if err == nil {
				if err = os.Chown(filepath.Dir(destpath), uid, gid); err != nil {
					return err
				}
			}
		}

		if st, err := os.Stat(destpath); err == nil {
			if !st.Mode().IsRegular() {
				log.Fatalln("dest exists and is not a plain file")
			}
			datetime := time.Now().UTC().Format("20060102150405")
			backuppath = destpath + "." + datetime + ".old"
			if err = os.Rename(destpath, backuppath); err != nil {
				return err
			}
		}
		out, err := os.Create(destpath)
		if err != nil {
			return err
		}
		defer out.Close()

		if err = out.Chown(uid, gid); err != nil {
			os.Remove(out.Name())
			if backuppath != "" {
				if err = os.Rename(backuppath, destpath); err != nil {
					return err
				}
				return err
			}
		}

		if _, err = io.Copy(out, from); err != nil {
			return err
		}
		return nil
	}
	return ErrNotSupported
}
