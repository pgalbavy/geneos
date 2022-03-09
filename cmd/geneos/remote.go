package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/sftp"
)

// remote support

const Remote Component = "remote"

// global to indicate current remote target. default to "local" which is a special case
// var remoteTarget = "local"
const LOCAL = "local"
const ALL = "all"

type Remotes struct {
	InstanceBase
	HomeDir  string `default:"{{join .InstanceRoot \"remotes\" .InstanceName}}"`
	Hostname string
	Port     int `default:"22"`
	Username string
	ITRSHome string            `default:"{{.InstanceRoot}}"`
	OSInfo   map[string]string `json:",omitempty"`
}

func init() {
	RegisterComponent(Components{
		New:              NewRemote,
		ComponentType:    Remote,
		ComponentMatches: []string{"remote", "remotes"},
		IncludeInLoops:   false,
		DownloadBase:     "",
	})
	RegisterDirs([]string{
		"remotes",
	})
	RegisterSettings(GlobalSettings{})
}

// interface method set

func (r Remotes) Type() Component {
	return parseComponentName(r.InstanceType)
}

func (r Remotes) Name() string {
	return r.InstanceName
}

func (r Remotes) Location() string {
	return r.InstanceLocation
}

func (r Remotes) Prefix(field string) string {
	return field
}

func (r Remotes) Home() string {
	return r.HomeDir
}

//
// 'geneos add remote NAME SSH-URL'
//
func (r Remotes) Create(username string, params []string) (err error) {
	if len(params) == 0 {
		logError.Fatalln("remote destination must be provided in the form of a URL")
	}

	u, err := url.Parse(params[0])
	if err != nil {
		logDebug.Println(err)
		return
	}

	if u.Scheme != "ssh" {
		logError.Fatalln("unsupported scheme (only ssh at the moment):", u.Scheme)
	}

	if u.Host == "" {
		logError.Fatalln("hostname must be provided")
	}
	r.Hostname = u.Host

	if u.Port() != "" {
		r.Port, _ = strconv.Atoi(u.Port())
	}

	if u.User.Username() != "" {
		username = u.User.Username()
	}
	r.Username = username

	homepath := ITRSHome()
	if u.Path != "" {
		homepath = u.Path
	}
	r.ITRSHome = homepath

	err = writeInstanceConfig(r)
	if err != nil {
		logError.Fatalln(err)
	}

	// once we are bootstrapped, read os-release info and re-write config
	err = r.getOSReleaseEnv()
	if err != nil {
		log.Fatalln(err)
	}

	err = writeInstanceConfig(r)
	if err != nil {
		logError.Fatalln(err)
	}

	// check and created file layout
	if _, err = statFile(r.Name(), homepath); err == nil {
		dirs, err := readDir(r.Name(), homepath)
		if err != nil {
			logError.Fatalln(err)
		}
		// ignore dot files
		for _, entry := range dirs {
			if !strings.HasPrefix(entry.Name(), ".") {
				// directory exists and contains non dot files/dirs - so return
				return nil
			}
		}
	} else {
		// need to create out own, chown base directory only
		if err = mkdirAll(r.Name(), homepath, 0775); err != nil {
			logError.Fatalln(err)
		}
	}

	// create dirs
	// create directories - initDirs is global, in main.go
	for _, d := range initDirs {
		dir := filepath.Join(homepath, d)
		if err = mkdirAll(r.Name(), dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}
	return
}

func (c Remotes) Command() (args, env []string) {
	return
}

func (c Remotes) Clean(purge bool, params []string) (err error) {
	return ErrNotSupported
}

func (c Remotes) Reload(params []string) (err error) {
	return ErrNotSupported
}

func NewRemote(name string) Instances {
	local, remote := splitInstanceName(name)
	if remote != LOCAL {
		logError.Fatalln("remote remotes not suported")
	}
	// Bootstrap
	c := &Remotes{}
	c.InstanceRoot = ITRSHome()
	c.InstanceType = Remote.String()
	c.InstanceName = local
	c.InstanceLocation = remote
	setDefaults(&c)
	return c
}

func (r *Remotes) getOSReleaseEnv() (err error) {
	r.OSInfo = make(map[string]string)
	f, err := readFile(r.Name(), "/etc/os-release")
	if err != nil {
		f, err = readFile(r.Name(), "/usr/lib/os-release")
		if err != nil {
			log.Fatalln("cannot open /etc/os-release or /usr/lib/os-releaae")
		}
	}

	releaseFile := bytes.NewBuffer(f)
	scanner := bufio.NewScanner(releaseFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		s := strings.SplitN(line, "=", 2)
		if len(s) != 2 {
			return ErrInvalidArgs
		}
		key, value := s[0], s[1]
		value = strings.Trim(value, "\"")
		r.OSInfo[key] = value
	}
	return
}

func loadRemoteConfig(remote string) (c Instances) {
	c = NewRemote(remote)
	if err := loadConfig(c, false); err != nil {
		logError.Fatalf("cannot open remote %q configuration file", remote)
	}
	return
}

// Return the base directory for a Component
func remoteRoot(remote string) string {
	switch remote {
	case LOCAL:
		return ITRSHome()
	default:
		i := loadRemoteConfig(remote)
		if err := loadConfig(i, false); err != nil {
			logError.Fatalf("cannot open remote %q configuration file", remote)
		}
		return getString(i, "ITRSHome")
	}
}

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
func allRemotes() (remotes []Instances) {
	remotes = append([]Instances{Remote.New(LOCAL)}, Remote.remoteInstances(LOCAL)...)
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

func readlink(remote, file string) (link string, err error) {
	switch remote {
	case LOCAL:
		return os.Readlink(file)
	default:
		s := sftpOpenSession(remote)
		return s.ReadLink(file)
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
		// use PosixRename to overwrite oldpath
		return s.PosixRename(oldpath, newpath)
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

func writeFile(remote string, name string, b []byte, perm os.FileMode) (err error) {
	switch remote {
	case LOCAL:
		return os.WriteFile(name, b, perm)
	default:
		s := sftpOpenSession(remote)
		var f *sftp.File
		f, err = s.Create(name)
		if err != nil {
			return
		}
		defer f.Close()
		f.Chmod(perm)
		_, err = f.Write(b)
		return
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

func statAndOpenFile(remote string, name string) (f io.ReadSeekCloser, st fileStat, err error) {
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
