package main

import (
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// remote support

// "remote" is another component type,

// e.g.
// geneos new remote X URL
//
// below is out of date

// examples:
//
// geneos add remote name URL
//   URL here may be ssh://user@host etc.
// URL can include path to ITRS_HOME / ITRSHome, e.g.
//   ssh://user@server/home/geneos
// else it default to same as local
//
// non ssh schemes to follow
//   ssh support for agents and private key files - no passwords
//   known_hosts will be checked, no changes made, missing keys will
//   result in error. user must add hosts before use (ssh-keyscan)
//
// geneos ls remote NAME
// XXX - geneos init remote NAME
//
// remote 'localhost' is always implied
//
// geneos ls remote
// ... list remote locations
//
// geneos start gateway [name]@remote
//
// XXX support gateway pairs for standby - how ?
//
// XXX remote netprobes, auto configure with gateway for SANs etc.?
//
// support existing geneos-utils installs on remote

type Remotes struct {
	Common
	Home     string `default:"{{join .Root \"remotes\" .Name}}"`
	Hostname string
	Port     int `default:"22"`
	Username string
	ITRSHome string `default:"{{.Root}}"`
}

func init() {
	components[Remote] = ComponentFuncs{
		Instance: remoteInstance,
		Command:  nil,
		Add:      remoteAdd,
		Clean:    nil,
		Reload:   nil,
	}
}

func remoteInstance(name string) interface{} {
	local, remote := splitInstanceName(name)
	if remote != LOCAL {
		logError.Fatalln("remote remotes not suported")
	}
	// Bootstrap
	c := &Remotes{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Remote.String()
	c.Name = local
	c.Rem = remote
	setDefaults(&c)
	return c
}

// Return the base directory for a ComponentType
func remoteRoot(remote string) string {
	switch remote {
	case LOCAL:
		return RunningConfig.ITRSHome
	default:
		i := remoteInstance(remote).(Instance)
		if err := loadConfig(i, false); err != nil {
			logError.Fatalln(err)
		}
		return getString(i, "ITRSHome")

	}
}

//
// 'geneos add remote NAME SSH-URL'
//
func remoteAdd(name string, username string, params []string) (c Instance, err error) {
	if len(params) == 0 {
		logError.Fatalln("remote destination must be provided in the form of a URL")
	}

	c = remoteInstance(name)

	u, err := url.Parse(params[0])
	if err != nil {
		logDebug.Println(err)
		return
	}

	switch {
	case u.Scheme == "ssh":
		if u.Host == "" {
			logError.Fatalln("hostname must be provided")
		}
		setField(c, "Hostname", u.Host)
		if u.Port() != "" {
			setField(c, "Port", u.Port())
		}
		if u.User.Username() != "" {
			setField(c, "Username", u.User.Username())
		}
		if u.Path != "" {
			setField(c, "ITRSHome", u.Path)
		}
		return c, writeInstanceConfig(c)
	default:
		logError.Fatalln("unsupport scheme (only ssh at the moment):", u.Scheme)
	}
	return
}

// global to indicate current remote target. default to "local" which is a special case
// var remoteTarget = "local"
const LOCAL = "local"

// given an instance name, split on an '@' and return left and right parts, using
// "local" as a default
func splitInstanceName(in string) (name, remote string) {
	remote = "local"
	parts := strings.SplitN(in, "@", 2)
	name = parts[0]
	if len(parts) > 1 {
		remote = parts[1]
	}
	return
}

// this is not recursive,
// but we include a special LOCAL instance
func allRemotes() (remotes []Instance) {
	remotes = newComponent(Remote, LOCAL)
	remotes = append(remotes, instancesOfComponent(LOCAL, Remote)...)
	return
}

// shim methods that test remote and direct to ssh / sftp / os

func mkdirAll(remote string, path string, perm os.FileMode) error {
	logDebug.Println("mkdirAll", remote, path, perm)
	switch remote {
	case LOCAL:
		return os.MkdirAll(path, perm)
	default:
		return ErrNotSupported
	}
}

func chown(remote string, name string, uid, gid int) error {
	logDebug.Println("chown", remote, name, uid, gid)

	switch remote {
	case LOCAL:
		return os.Chown(name, uid, gid)
	default:
		return ErrNotSupported
	}
}

func createFile(remote string, path string) (*os.File, error) {
	logDebug.Println("createFile", remote, path)

	switch remote {
	case LOCAL:
		return os.Create(path)
	default:
		return nil, ErrNotSupported
	}
}

func removeFile(remote string, name string) error {
	logDebug.Println("removeFile", remote, name)

	switch remote {
	case LOCAL:
		return os.Remove(name)
	default:
		return ErrNotSupported
	}
}

func removeAll(remote string, name string) error {
	logDebug.Println("removeFile", remote, name)

	switch remote {
	case LOCAL:
		return os.RemoveAll(name)
	default:
		return ErrNotSupported
	}
}

func renameFile(remote string, oldpath, newpath string) error {
	logDebug.Println("renameFile", remote, oldpath, newpath)

	switch remote {
	case LOCAL:
		return os.Rename(oldpath, newpath)
	default:
		return ErrNotSupported
	}
}

func statFile(remote string, name string) (st os.FileInfo, err error) {
	logDebug.Println("statFile", remote, name)

	switch remote {
	case LOCAL:
		return os.Stat(name)
	default:
		s, err := sftpOpenSession(remote)
		if err != nil {
			logError.Fatalln(err)
		}
		return s.Stat(name)
	}
}

func globPath(remote string, pattern string) ([]string, error) {
	logDebug.Println("globPath", remote, pattern)

	switch remote {
	case LOCAL:
		return filepath.Glob(pattern)
	default:
		s, err := sftpOpenSession(remote)
		if err != nil {
			logError.Fatalln(err)
		}
		return s.Glob(pattern)
	}
}

func readFile(remote string, name string) (b []byte, err error) {
	// logDebug.Println("readFile", remote, name)

	switch remote {
	case LOCAL:
		return os.ReadFile(name)
	default:
		s, err := sftpOpenSession(remote)
		if err != nil {
			logError.Fatalln(err)
		}
		f, err := s.Open(name)
		if err != nil {
			logError.Fatalln(err)
		}
		defer f.Close()

		st, err := f.Stat()
		if err != nil {
			logError.Fatalln(err)
		}
		b = make([]byte, st.Size())
		_, err = io.ReadFull(f, b)
		return b, err
	}
}

func readDir(remote string, name string) (dirs []os.DirEntry, err error) {
	// logDebug.Printf("readDir %q %q", remote, name)

	switch remote {
	case LOCAL:
		return os.ReadDir(name)
	default:
		s, err := sftpOpenSession(remote)
		if err != nil {
			logError.Fatalln(err)
		}
		f, err := s.ReadDir(name)
		if err != nil {
			return nil, err
		}
		for _, d := range f {
			dirs = append(dirs, fs.FileInfoToDirEntry(d))
		}
	}
	return
}

func openFile(remote string, name string) (*os.File, error) {
	logDebug.Printf("%q %q", remote, name)

	switch remote {
	case LOCAL:
		return os.Open(name)
	default:
		return nil, ErrNotSupported
	}
}
