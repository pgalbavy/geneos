package host

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/utils"
)

// remote support

// const Remote Component = "remote"

// global to indicate current remote target. default to "local" which is a special case
// var remoteTarget = "local"

type Name string

const LOCALHOST Name = "localhost"
const ALLHOSTS Name = "all"

var LOCAL, ALL *Host

type Host struct {
	// instance.Instance
	V *viper.Viper `json:"-"`

	name     Name              `json:"Name,omitempty"` // name, as opposed to hostname
	dir      string            `json:"HomeDir,omitempty"`
	hostname string            `json:"Hostname,omitempty"`
	port     int               `default:"22" json:"Port,omitempty"`
	username string            `json:"Username,omitempty"`
	geneos   string            `json:"Geneos,omitempty"` // Geneos root directory
	osinfo   map[string]string `json:"OSInfo,omitempty"`
}

func init() {
	// component.RegisterComponent(component.Components{
	// 	New:              NewRemote,
	// 	ComponentType:    Remote,
	// 	ComponentMatches: []string{"remote", "remotes"},
	// 	RealComponent:    false,
	// 	DownloadBase:     "",
	// })
	// Remote.RegisterDirs([]string{
	// 	"remotes",
	// })
	// component.RegisterDefaultSettings(GlobalSettings{})

}

// interface method set

// cache instances of remotes as they get used frequently
// var remotes map[RemoteName]*Remotes = make(map[RemoteName]*Remotes)
var remotes sync.Map

func New(name Name) interface{} {
	localpart, remotepart := splitInstanceName(name)
	if remotepart != LOCALHOST {
		logDebug.Println("remote remotes not supported")
		return nil
	}
	r, ok := remotes.Load(localpart)
	if ok {
		rem, ok := r.(*Host)
		if ok {
			return rem
		}
	}

	// Bootstrap
	c := new(Host)
	// c.InstanceRemote = rLOCAL
	// c.RemoteRoot = Geneos()
	// c.InstanceType = Remote.String()
	// c.InstanceName = localpart
	// c.L = new(sync.RWMutex)
	// if err := setDefaults(&c); err != nil {
	// 	logError.Fatalln(c, "setDefaults():", err)
	// }
	// c.InstanceLocation = LOCAL
	// fill this in directly as there is no config file to load
	// if c.RemoteName() == LOCAL {
	// 	c.getOSReleaseEnv()
	// }
	// these are pseudo remotes and always exist
	// if c.InstanceName == string(LOCAL) || c.InstanceName == string(ALL) {
	// 	c.ConfigLoaded = true
	// }
	remotes.Store(localpart, c)
	return c
}

func (host Name) String() string {
	return string(host)
}

func (h *Host) String() string {
	return string(h.name)
}

//
// 'geneos add remote NAME [SSH-URL] [init opts]'
//
func (r *Host) Add(username string, params []string, tmpl string) (err error) {
	if len(params) == 0 {
		// default - try ssh to a host with the same name as remote
		params = []string{"ssh://" + string(r.name)}
	}

	var remurl string
	if strings.HasPrefix(params[0], "ssh://") {
		remurl = params[0]
		params = params[1:]
	} else if strings.HasPrefix(params[0], "/") {
		remurl = "ssh://" + r.String() + params[0]
		params = params[1:]
	} else {
		remurl = "ssh://" + r.String()
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
	r.hostname = u.Host
	if r.hostname == "" {
		r.hostname = string(r.name)
	}

	if u.Port() != "" {
		r.port, _ = strconv.Atoi(u.Port())
	}

	if u.User.Username() != "" {
		username = u.User.Username()
	}
	r.username = username

	// XXX default to remote user's home dir, not local
	r.geneos = Geneos()
	if u.Path != "" {
		// XXX check and adopt local setting for remote user and/or remote global settings
		// - only if ssh URL does not contain explicit path
		r.geneos = u.Path
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
		if err = commandSet(Remote, []string{r.String()}, params); err != nil {
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

func (r *Host) Command() (args, env []string) {
	return
}

func (r *Host) Reload(params []string) (err error) {
	return ErrNotSupported
}

func (r *Host) Rebuild(initial bool) error {
	return ErrNotSupported
}

func (h *Host) getOSReleaseEnv() (err error) {
	h.osinfo = make(map[string]string)
	f, err := h.ReadFile("/etc/os-release")
	if err != nil {
		if f, err = h.ReadFile("/usr/lib/os-release"); err != nil {
			return fmt.Errorf("cannot open /etc/os-release or /usr/lib/os-release")
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
		h.OSInfo[key] = value
	}
	return
}

func GetRemote(remote Name) (r *Host) {
	switch remote {
	case LOCALHOST:
		return LOCAL
	case ALLHOSTS:
		return ALL
	default:
		i := New(string(remote))
		i.Load()
		return i.(*Host)
	}
}

// Return the base directory for the remote, inc LOCAL
func (r *Host) GeneosRoot() string {
	return r.geneos
}

// return an absolute path anchored in the root directory of the remote
// this can also be LOCAL
func (r *Host) GeneosPath(paths ...string) string {
	return filepath.Join(append([]string{r.GeneosRoot()}, paths...)...)
}

func (r *Host) FullName(name string) string {
	if strings.Contains(name, "@") {
		return name
	}
	return name + "@" + r.String()
}

func AllHosts() (remotes []*Host) {
	remotes = []*Host{LOCAL}
	if utils.IsSuperuser {
		return
	}
	// for _, r := range GetInstancesForComponent(LOCAL, component.Remote) {
	// 	remotes = append(remotes, r.(*Remotes))
	// }
	return
}
