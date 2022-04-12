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
	"sync"
	"syscall"

	"github.com/pkg/sftp"
)

// remote support

const Remote Component = "remote"

// global to indicate current remote target. default to "local" which is a special case
// var remoteTarget = "local"

type RemoteName string

const LOCAL RemoteName = "local"
const ALL RemoteName = "all"

var rLOCAL, rALL *Remotes

type Remotes struct {
	InstanceBase
	HomeDir  string `default:"{{join .RemoteRoot \"remotes\" .InstanceName}}"`
	Hostname string
	Port     int `default:"22"`
	Username string
	ITRSHome string
	Geneos   string            `default:"{{.RemoteRoot}}"`
	OSInfo   map[string]string `json:",omitempty"`
}

func init() {
	RegisterComponent(Components{
		New:              NewRemote,
		ComponentType:    Remote,
		ComponentMatches: []string{"remote", "remotes"},
		RealComponent:    false,
		DownloadBase:     "",
	})
	Remote.RegisterDirs([]string{
		"remotes",
	})
	RegisterDefaultSettings(GlobalSettings{})

}

// interface method set

// cache instances of remotes as they get used frequently
// var remotes map[RemoteName]*Remotes = make(map[RemoteName]*Remotes)
var remotes sync.Map

func NewRemote(name string) Instance {
	localpart, remotepart := splitInstanceName(name)
	if remotepart != LOCAL {
		logDebug.Println("remote remotes not suported")
		return nil
	}
	r, ok := remotes.Load(localpart)
	if ok {
		rem, ok := r.(*Remotes)
		if ok {
			return rem
		}
	}

	// Bootstrap
	c := new(Remotes)
	c.InstanceRemote = rLOCAL
	c.RemoteRoot = Geneos()
	c.InstanceType = Remote.String()
	c.InstanceName = localpart
	c.L = new(sync.RWMutex)
	if err := setDefaults(&c); err != nil {
		logError.Fatalln(c, "setDefaults():", err)
	}
	c.InstanceLocation = LOCAL
	// fill this in directly as there is no config file to load
	if c.RemoteName() == LOCAL {
		c.getOSReleaseEnv()
	}
	// these are pseudo remotes and always exist
	if c.InstanceName == string(LOCAL) || c.InstanceName == string(ALL) {
		c.ConfigLoaded = true
	}
	remotes.Store(localpart, c)
	return c
}

func (r *Remotes) Type() Component {
	return parseComponentName(r.InstanceType)
}

func (r *Remotes) Name() string {
	return r.InstanceName
}

func (r *Remotes) Location() RemoteName {
	return r.InstanceLocation
}

func (r *Remotes) Prefix(field string) string {
	return field
}

func (r *Remotes) Home() string {
	return r.HomeDir
}

func (remote RemoteName) String() string {
	return string(remote)
}

func (r *Remotes) Load() (err error) {
	if r.ConfigLoaded {
		return
	}
	err = loadConfig(r)
	// convert
	if err == nil && r.ITRSHome != "" {
		r.Geneos = r.ITRSHome
		r.ITRSHome = ""
		if err = writeInstanceConfig(r); err != nil {
			log.Printf("%s: cannot write remote configuration file. Contents will be out of sync.", r)
		}
	}
	r.ConfigLoaded = err == nil
	return
}

func (r *Remotes) Unload() (err error) {
	if r == rLOCAL || r == rALL {
		remotes.Delete(r.Name())
		return
	}
	r.ConfigLoaded = false
	return
}

func (r *Remotes) Loaded() bool {
	return r.ConfigLoaded
}

func (r *Remotes) RemoteName() RemoteName {
	return RemoteName(r.InstanceName)
}

func (r *Remotes) Remote() *Remotes {
	return r.InstanceRemote
}

func (r *Remotes) Base() *InstanceBase {
	return &r.InstanceBase
}

//
// 'geneos add remote NAME [SSH-URL] [init opts]'
//
func (r *Remotes) Add(username string, params []string, tmpl string) (err error) {
	if len(params) == 0 {
		// default - try ssh to a host with the same name as remote
		params = []string{"ssh://" + r.Name()}
	}

	var remurl string
	if strings.HasPrefix(params[0], "ssh://") {
		remurl = params[0]
		params = params[1:]
	} else if strings.HasPrefix(params[0], "/") {
		remurl = "ssh://" + r.Name() + params[0]
		params = params[1:]
	} else {
		remurl = "ssh://" + r.Name()
	}

	if err = initFlagSet.Parse(params); err != nil {
		return
	}

	u, err := url.Parse(remurl)
	if err != nil {
		return
	}

	if u.Scheme != "ssh" {
		return fmt.Errorf("unsupported scheme (only ssh at the moment): %q", u.Scheme)
	}

	// if no hostname in URL fall back to remote name (e.g. ssh:///path)
	r.Hostname = u.Host
	if r.Hostname == "" {
		r.Hostname = r.Name()
	}

	if u.Port() != "" {
		r.Port, _ = strconv.Atoi(u.Port())
	}

	if u.User.Username() != "" {
		username = u.User.Username()
	}
	r.Username = username

	// XXX default to remote user's home dir, not local
	r.Geneos = Geneos()
	if u.Path != "" {
		// XXX check and adopt local setting for remote user and/or remote global settings
		// - only if ssh URL does not contain explicit path
		r.Geneos = u.Path
	}
	// r.Geneos = homepath

	if err = writeInstanceConfig(r); err != nil {
		return
	}

	// once we are bootstrapped, read os-release info and re-write config
	if err = r.getOSReleaseEnv(); err != nil {
		return
	}

	if err = writeInstanceConfig(r); err != nil {
		return
	}

	// apply any extra args to settings
	if len(params) > 0 {
		if err = commandSet(Remote, []string{r.Name()}, params); err != nil {
			return
		}
		r.Unload()
		r.Load()
	}

	// initialise the remote directory structure, but perhaps ignore errors
	// as we may simply be adding an existing installation
	if err = r.initGeneos([]string{r.Geneos}); err != nil {
		return err
	}

	for _, c := range components {
		if c.Initialise != nil {
			c.Initialise(r)
		}
	}

	return
}

func (r *Remotes) Command() (args, env []string) {
	return
}

func (r *Remotes) Reload(params []string) (err error) {
	return ErrNotSupported
}

func (r *Remotes) Rebuild(initial bool) error {
	return ErrNotSupported
}

func (r *Remotes) getOSReleaseEnv() (err error) {
	r.OSInfo = make(map[string]string)
	f, err := r.ReadFile("/etc/os-release")
	if err != nil {
		if f, err = r.ReadFile("/usr/lib/os-release"); err != nil {
			return fmt.Errorf("cannot open /etc/os-release or /usr/lib/os-releaae")
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

func GetRemote(remote RemoteName) (r *Remotes) {
	switch remote {
	case LOCAL:
		return rLOCAL
	case ALL:
		return rALL
	default:
		i := NewRemote(string(remote))
		i.Load()
		return i.(*Remotes)
	}
}

// Return the base directory for the remote, inc LOCAL
func (r *Remotes) GeneosRoot() string {
	return r.Geneos
}

// return an absolute path anchored in the root directory of the remote
// this can also be LOCAL
func (r *Remotes) GeneosPath(paths ...string) string {
	return filepath.Join(append([]string{r.GeneosRoot()}, paths...)...)
}

func (r *Remotes) FullName(name string) string {
	if strings.Contains(name, "@") {
		return name
	}
	return name + "@" + r.InstanceName
}

func (r *Remotes) String() string {
	return r.Type().String() + ":" + r.InstanceName + "@" + r.Location().String()
}

// given an instance name, split on an '@' and return left and right parts, using
// "local" as a default
func splitInstanceName(in string) (name string, remote RemoteName) {
	remote = LOCAL
	parts := strings.SplitN(in, "@", 2)
	name = parts[0]
	if len(parts) > 1 {
		remote = RemoteName(parts[1])
	}
	return
}

// add an optional component type prefix and colon sep
//
func SplitInstanceName(in string, defaultRemote *Remotes) (ct Component, name string, r *Remotes) {
	r = defaultRemote
	parts := strings.SplitN(in, "@", 2)
	name = parts[0]
	if len(parts) > 1 {
		r = GetRemote(RemoteName(parts[1]))
	}
	parts = strings.SplitN(name, ":", 2)
	if len(parts) > 1 {
		ct = parseComponentName(parts[0])
		name = parts[1]
	}
	return
}

func AllRemotes() (remotes []*Remotes) {
	remotes = []*Remotes{rLOCAL}
	if superuser {
		return
	}
	for _, r := range Remote.GetInstancesForComponent(rLOCAL) {
		remotes = append(remotes, r.(*Remotes))
	}
	return
}

// shim methods that test remote and direct to ssh / sftp / os
// at some point this should become interface based to allow other
// remote protocols cleanly

func (r *Remotes) Symlink(target, path string) (err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.Symlink(target, path)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
		return s.Symlink(target, path)
	}
}

func (r *Remotes) ReadLink(file string) (link string, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.Readlink(file)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
		return s.ReadLink(file)
	}
}

func (r *Remotes) MkdirAll(path string, perm os.FileMode) (err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.MkdirAll(path, perm)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
		return s.MkdirAll(path)
	}
}

func (r *Remotes) Chown(name string, uid, gid int) (err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.Chown(name, uid, gid)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
		return s.Chown(name, uid, gid)
	}
}

func (r *Remotes) Create(path string, perms fs.FileMode) (out io.WriteCloser, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		var cf *os.File
		cf, err = os.Create(path)
		if err != nil {
			return
		}
		out = cf
		if err = cf.Chmod(perms); err != nil {
			return
		}
	default:
		var cf *sftp.File
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
		if cf, err = s.Create(path); err != nil {
			return
		}
		out = cf
		if err = cf.Chmod(perms); err != nil {
			return
		}
	}
	return
}

func (r *Remotes) Remove(name string) (err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.Remove(name)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
		return s.Remove(name)
	}
}

func (r *Remotes) RemoveAll(name string) (err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.RemoveAll(name)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}

		// walk, reverse order by prepending and remove
		// we could also just reverse sort strings...
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
				return
			}
		}
		return
	}
}

func (r *Remotes) Rename(oldpath, newpath string) (err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.Rename(oldpath, newpath)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
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
func (r *Remotes) Stat(name string) (s fileStat, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		if s.st, err = os.Stat(name); err != nil {
			return
		}
		s.uid = s.st.Sys().(*syscall.Stat_t).Uid
		s.gid = s.st.Sys().(*syscall.Stat_t).Gid
		s.mtime = s.st.Sys().(*syscall.Stat_t).Mtim.Sec
	default:
		var sf *sftp.Client
		if sf, err = r.sftpOpenSession(); err != nil {
			return
		}
		if s.st, err = sf.Stat(name); err != nil {
			return
		}
		s.uid = s.st.Sys().(*sftp.FileStat).UID
		s.gid = s.st.Sys().(*sftp.FileStat).GID
		s.mtime = int64(s.st.Sys().(*sftp.FileStat).Mtime)
	}
	return
}

// lstat() a local or remote file and normalise common values
func (r *Remotes) Lstat(name string) (s fileStat, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		if s.st, err = os.Lstat(name); err != nil {
			return
		}
		s.uid = s.st.Sys().(*syscall.Stat_t).Uid
		s.gid = s.st.Sys().(*syscall.Stat_t).Gid
		s.mtime = s.st.Sys().(*syscall.Stat_t).Mtim.Sec
	default:
		var sf *sftp.Client
		if sf, err = r.sftpOpenSession(); err != nil {
			return
		}
		if s.st, err = sf.Lstat(name); err != nil {
			return
		}
		s.uid = s.st.Sys().(*sftp.FileStat).UID
		s.gid = s.st.Sys().(*sftp.FileStat).GID
		s.mtime = int64(s.st.Sys().(*sftp.FileStat).Mtime)
	}
	return
}

func (r *Remotes) Glob(pattern string) (paths []string, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return filepath.Glob(pattern)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
		return s.Glob(pattern)
	}
}

func (r *Remotes) WriteFile(path string, b []byte, perm os.FileMode) (err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.WriteFile(path, b, perm)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
		var f *sftp.File
		if f, err = s.Create(path); err != nil {
			return
		}
		defer f.Close()
		f.Chmod(perm)
		_, err = f.Write(b)
		return
	}
}

func (r *Remotes) ReadFile(name string) (b []byte, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.ReadFile(name)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
		f, err := s.Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		st, err := f.Stat()
		if err != nil {
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

func (r *Remotes) ReadDir(name string) (dirs []os.DirEntry, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.ReadDir(name)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
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

func (r *Remotes) Open(name string) (f io.ReadSeekCloser, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		f, err = os.Open(name)
	default:
		var s *sftp.Client
		if s, err = r.sftpOpenSession(); err != nil {
			return
		}
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
func (r *Remotes) createTempFile(path string, perms fs.FileMode) (f io.WriteCloser, name string, err error) {
	try := 0
	for {
		name = path + nextRandom()
		f, err = r.Create(name, perms)
		if os.IsExist(err) {
			if try++; try < 100 {
				continue
			}
			return nil, "", fs.ErrExist
		}
		return
	}
}
