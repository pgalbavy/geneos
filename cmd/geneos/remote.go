package main

import (
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/sftp"
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
	c.Location = remote
	setDefaults(&c)
	return c
}

func loadRemoteConfig(remote string) (c Instance) {
	c = remoteInstance(remote).(Instance)
	if err := loadConfig(c, false); err != nil {
		logError.Fatalf("cannot open remote %q configuration file", remote)
	}
	return
}

// Return the base directory for a ComponentType
func remoteRoot(remote string) string {
	switch remote {
	case LOCAL:
		return RunningConfig.ITRSHome
	default:
		i := loadRemoteConfig(remote)
		if err := loadConfig(i, false); err != nil {
			logError.Fatalf("cannot open remote %q configuration file", remote)
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
const ALL = "all"

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

func symlink(remote string, oldname, newname string) error {
	switch remote {
	case LOCAL:
		return os.Symlink(oldname, newname)
	default:
		s := sftpOpenSession(remote)
		return s.Symlink(oldname, newname)
	}
}
func mkdirAll(remote string, path string, perm os.FileMode) error {
	switch remote {
	case LOCAL:
		return os.MkdirAll(path, perm)
	default:
		s := sftpOpenSession(remote)
		return s.MkdirAll(path)
	}
}

func chown(remote string, name string, uid, gid int) error {
	switch remote {
	case LOCAL:
		return os.Chown(name, uid, gid)
	default:
		s := sftpOpenSession(remote)
		return s.Chown(name, uid, gid)
	}
}

func createRemoteFile(remote string, path string) (*sftp.File, error) {
	switch remote {
	case LOCAL:
		return nil, ErrNotSupported
	default:
		s := sftpOpenSession(remote)
		return s.Create(path)
	}
}

func removeFile(remote string, name string) error {
	switch remote {
	case LOCAL:
		return os.Remove(name)
	default:
		s := sftpOpenSession(remote)
		return s.Remove(name)
	}
}

func removeAll(remote string, name string) (err error) {
	switch remote {
	case LOCAL:
		return os.RemoveAll(name)
	default:
		s := sftpOpenSession(remote)

		// walk, reverse order by prepending and remove
		files := []string{}
		w := s.Walk(name)
		for w.Step() {
			if w.Err() != nil {
				continue
			}
			files = append([]string{w.Path()}, files...)
		}
		for _, file := range files {
			if err = s.Remove(file); err != nil {
				log.Println("remove failed", err)
				return
			}
		}
		return
	}
}

func renameFile(remote string, oldpath, newpath string) error {
	switch remote {
	case LOCAL:
		return os.Rename(oldpath, newpath)
	default:
		s := sftpOpenSession(remote)
		return s.Rename(oldpath, newpath)
	}
}

// massaged file stats
type fileStat struct {
	st    os.FileInfo
	uid   uint32
	gid   uint32
	mtime int64
}

// stat() a local or remote file and normalise common values
func statFile(remote string, name string) (s fileStat, err error) {
	switch remote {
	case LOCAL:
		s.st, err = os.Stat(name)
		if err != nil {
			return
		}
		s.uid = s.st.Sys().(*syscall.Stat_t).Uid
		s.gid = s.st.Sys().(*syscall.Stat_t).Gid
		s.mtime = s.st.Sys().(*syscall.Stat_t).Mtim.Sec
	default:
		sf := sftpOpenSession(remote)
		s.st, err = sf.Stat(name)
		if err != nil {
			return
		}
		s.uid = s.st.Sys().(*sftp.FileStat).UID
		s.gid = s.st.Sys().(*sftp.FileStat).GID
		s.mtime = int64(s.st.Sys().(*sftp.FileStat).Mtime)
	}
	return
}

func globPath(remote string, pattern string) ([]string, error) {
	switch remote {
	case LOCAL:
		return filepath.Glob(pattern)
	default:
		s := sftpOpenSession(remote)
		return s.Glob(pattern)
	}
}

func readFile(remote string, name string) ([]byte, error) {
	switch remote {
	case LOCAL:
		return os.ReadFile(name)
	default:
		s := sftpOpenSession(remote)
		f, err := s.Open(name)
		if err != nil {
			// logError.Fatalln(err)
			return nil, err
		}
		defer f.Close()

		st, err := f.Stat()
		if err != nil {
			// logError.Fatalln(err)
			return nil, err
		}
		// force a block read as /proc doesn't give sizes
		sz := st.Size()
		if sz == 0 {
			sz = 8192
		}
		return io.ReadAll(f)
	}
}

func readDir(remote string, name string) (dirs []os.DirEntry, err error) {
	switch remote {
	case LOCAL:
		return os.ReadDir(name)
	default:
		s := sftpOpenSession(remote)
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

func openStatFile(remote string, name string) (f io.ReadSeekCloser, st fileStat, err error) {
	st, err = statFile(remote, name)
	if err != nil {
		return
	}
	switch remote {
	case LOCAL:
		f, err = os.Open(name)
	default:
		s := sftpOpenSession(remote)
		f, err = s.Open(name)
	}
	return
}

func nextRandom() string {
	return fmt.Sprint(rand.Uint32())
}

// based on os.CreatTemp, but allows for remotes and much simplified
// given a remote and a full path, create a file with a suffix
// and return an io.File
func createRemoteTemp(remote string, path string) (*sftp.File, error) {
	try := 0
	for {
		name := path + nextRandom()
		f, err := createRemoteFile(remote, name)
		if os.IsExist(err) {
			if try++; try < 100 {
				continue
			}
			return nil, fs.ErrExist
		}
		return f, err
	}
}
