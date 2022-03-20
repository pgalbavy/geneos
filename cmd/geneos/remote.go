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
	"golang.org/x/crypto/ssh"
)

// remote support

const Remote Component = "remote"

// global to indicate current remote target. default to "local" which is a special case
// var remoteTarget = "local"

type RemoteName string

const LOCAL RemoteName = "local"
const ALL RemoteName = "all"

var rLOCAL, rALL *Remotes

// cache instances of remotes as they get used frequently
var remotes map[RemoteName]*Remotes = make(map[RemoteName]*Remotes)

type Remotes struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client

	InstanceBase
	HomeDir  string `default:"{{join .RemoteRoot \"remotes\" .InstanceName}}"`
	Hostname string
	Port     int `default:"22"`
	Username string
	ITRSHome string            `default:"{{.RemoteRoot}}"`
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
	RegisterDirs([]string{
		"remotes",
	})
	RegisterSettings(GlobalSettings{})

}

// interface method set

func NewRemote(name string) Instances {
	local, remote := splitInstanceName(name)
	if remote != LOCAL {
		logError.Fatalln("remote remotes not suported")
	}
	r, ok := remotes[RemoteName(local)]
	if ok {
		return r
	}
	// Bootstrap
	c := &Remotes{}
	c.InstanceRemote = rLOCAL
	c.RemoteRoot = ITRSHome()
	c.InstanceType = Remote.String()
	c.InstanceName = local
	setDefaults(&c)
	c.InstanceLocation = LOCAL
	// fill this in directly as there is no config file to load
	if c.RemoteName() == LOCAL {
		c.getOSReleaseEnv()
	}
	// these are pseudo remotes and always exist
	if c.InstanceName == string(LOCAL) || c.InstanceName == string(ALL) {
		c.ConfigLoaded = true
	}
	remotes[RemoteName(local)] = c
	return c
}

func (r Remotes) Type() Component {
	return parseComponentName(r.InstanceType)
}

func (r Remotes) Name() string {
	return r.InstanceName
}

func (r Remotes) Location() RemoteName {
	return r.InstanceLocation
}

func (r Remotes) Prefix(field string) string {
	return field
}

func (r Remotes) Home() string {
	return r.HomeDir
}

func (remote RemoteName) String() string {
	return string(remote)
}

func (r Remotes) Load() (err error) {
	if r.ConfigLoaded {
		return
	}
	err = loadConfig(r)
	r.ConfigLoaded = err == nil
	return
}

func (r Remotes) Unload() (err error) {
	if &r == rLOCAL || &r == rALL {
		return ErrInvalidArgs
	}
	r.ConfigLoaded = false
	return
}

func (r Remotes) Loaded() bool {
	return r.ConfigLoaded
}

func (r Remotes) RemoteName() RemoteName {
	return RemoteName(r.InstanceName)
}

func (r Remotes) Remote() *Remotes {
	return r.InstanceRemote
}

//
// 'geneos add remote NAME [SSH-URL] [init opts]'
//
func (r Remotes) Add(username string, params []string, tmpl string) (err error) {
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
		log.Fatalln(err)
	}

	u, err := url.Parse(remurl)
	if err != nil {
		logDebug.Println(err)
		return
	}

	if u.Scheme != "ssh" {
		logError.Fatalln("unsupported scheme (only ssh at the moment):", u.Scheme)
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

	// default to remote user's home dir?
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

	if err = writeInstanceConfig(r); err != nil {
		logError.Fatalln(err)
	}

	// apply any extra args to settings
	if len(params) > 0 {
		commandSet(Remote, []string{r.Name()}, params)
		loadConfig(&r)
	}

	if err = r.initGeneos([]string{homepath}); err != nil {
		log.Fatalln(err)
	}

	for _, c := range components {
		if c.Initialise != nil {
			c.Initialise(&r)
		}
	}

	return
}

func (r Remotes) Command() (args, env []string) {
	return
}

func (r Remotes) Clean(purge bool, params []string) (err error) {
	return ErrNotSupported
}

func (r Remotes) Reload(params []string) (err error) {
	return ErrNotSupported
}

func (r Remotes) Rebuild(initial bool) error {
	return ErrNotSupported
}

func (r *Remotes) getOSReleaseEnv() (err error) {
	r.OSInfo = make(map[string]string)
	f, err := r.readFile("/etc/os-release")
	if err != nil {
		f, err = r.readFile("/usr/lib/os-release")
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

func GetRemote(remote RemoteName) (r *Remotes) {
	switch remote {
	case LOCAL:
		return rLOCAL
	case ALL:
		return rALL
	default:
		i := NewRemote(string(remote))
		loadConfig(i)
		return i.(*Remotes)
	}
}

// Return the base directory for the remote, inc LOCAL
func (r Remotes) GeneosRoot() string {
	return r.ITRSHome
}

// return an absolute path anchored in the root directory of the remote
// this can also be LOCAL
func (r Remotes) GeneosPath(paths ...string) string {
	return filepath.Join(append([]string{r.GeneosRoot()}, paths...)...)
}

func (r Remotes) String() string {
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

func SplitInstanceName(in string, defaultRemote *Remotes) (name string, r *Remotes) {
	r = defaultRemote
	parts := strings.SplitN(in, "@", 2)
	name = parts[0]
	if len(parts) > 1 {
		r = GetRemote(RemoteName(parts[1]))
	}
	return
}

func AllRemotes() (remotes []*Remotes) {
	remotes = []*Remotes{rLOCAL}
	for _, r := range Remote.Instances(rLOCAL) {
		remotes = append(remotes, r.(*Remotes))
	}
	return
}

// shim methods that test remote and direct to ssh / sftp / os

func (r *Remotes) symlink(oldname, newname string) error {
	switch r.InstanceName {
	case string(LOCAL):
		return os.Symlink(oldname, newname)
	default:
		s := r.sftpOpenSession()
		return s.Symlink(oldname, newname)
	}
}

func (r *Remotes) readlink(file string) (link string, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.Readlink(file)
	default:
		s := r.sftpOpenSession()
		return s.ReadLink(file)
	}
}

func (r *Remotes) mkdirAll(path string, perm os.FileMode) error {
	switch r.InstanceName {
	case string(LOCAL):
		return os.MkdirAll(path, perm)
	default:
		s := r.sftpOpenSession()
		return s.MkdirAll(path)
	}
}

func (r *Remotes) chown(name string, uid, gid int) error {
	switch r.InstanceName {
	case string(LOCAL):
		return os.Chown(name, uid, gid)
	default:
		s := r.sftpOpenSession()
		return s.Chown(name, uid, gid)
	}
}

func (r *Remotes) createFile(path string, perms fs.FileMode) (out io.WriteCloser, err error) {
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
		s := r.sftpOpenSession()
		cf, err = s.Create(path)
		if err != nil {
			return
		}
		out = cf
		if err = cf.Chmod(perms); err != nil {
			return
		}
	}
	return
}

func (r *Remotes) removeFile(name string) error {
	switch r.InstanceName {
	case string(LOCAL):
		return os.Remove(name)
	default:
		s := r.sftpOpenSession()
		return s.Remove(name)
	}
}

func (r *Remotes) removeAll(name string) (err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.RemoveAll(name)
	default:
		s := r.sftpOpenSession()

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

func (r *Remotes) renameFile(oldpath, newpath string) error {
	switch r.InstanceName {
	case string(LOCAL):
		return os.Rename(oldpath, newpath)
	default:
		s := r.sftpOpenSession()
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
func (r *Remotes) statFile(name string) (s fileStat, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		s.st, err = os.Stat(name)
		if err != nil {
			return
		}
		s.uid = s.st.Sys().(*syscall.Stat_t).Uid
		s.gid = s.st.Sys().(*syscall.Stat_t).Gid
		s.mtime = s.st.Sys().(*syscall.Stat_t).Mtim.Sec
	default:
		sf := r.sftpOpenSession()
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

func (r *Remotes) globPath(pattern string) ([]string, error) {
	switch r.InstanceName {
	case string(LOCAL):
		return filepath.Glob(pattern)
	default:
		s := r.sftpOpenSession()
		return s.Glob(pattern)
	}
}

func (r *Remotes) writeFile(path string, b []byte, perm os.FileMode) (err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.WriteFile(path, b, perm)
	default:
		s := r.sftpOpenSession()
		var f *sftp.File
		f, err = s.Create(path)
		if err != nil {
			return
		}
		defer f.Close()
		f.Chmod(perm)
		_, err = f.Write(b)
		return
	}
}

func (r *Remotes) readFile(name string) ([]byte, error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.ReadFile(name)
	default:
		s := r.sftpOpenSession()
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

func (r *Remotes) readDir(name string) (dirs []os.DirEntry, err error) {
	switch r.InstanceName {
	case string(LOCAL):
		return os.ReadDir(name)
	default:
		s := r.sftpOpenSession()
		f, err := s.ReadDir(name)
		logDebug.Println(name, f)
		if err != nil {
			return nil, err
		}
		for _, d := range f {
			dirs = append(dirs, fs.FileInfoToDirEntry(d))
		}
	}
	return
}

func (r *Remotes) statAndOpenFile(name string) (f io.ReadSeekCloser, st fileStat, err error) {
	st, err = r.statFile(name)
	if err != nil {
		return
	}
	switch r.InstanceName {
	case string(LOCAL):
		f, err = os.Open(name)
	default:
		s := r.sftpOpenSession()
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
		f, err = r.createFile(name, perms)
		if os.IsExist(err) {
			if try++; try < 100 {
				continue
			}
			return nil, "", fs.ErrExist
		}
		return
	}
}
