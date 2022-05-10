package host

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
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
	Name         Name   `json:"Name,omitempty"`    // name, as opposed to hostname
	Home         string `json:"HomeDir,omitempty"` // Remote configuration directory
	ConfigLoaded bool   `json:"-"`
	// Geneos string `json:"Geneos,omitempty"` // Geneos root directory

	Conf *viper.Viper `json:"-"`
}

// this is called from cmd root
func Init() {
	LOCAL = New(LOCALHOST)
	ALL = New(ALLHOSTS)
}

// return the absolute path to the local Geneos installation
func Geneos() string {
	home := viper.GetString("geneos")
	if home == "" {
		// fallback to support breaking change
		return viper.GetString("itrshome")
	}
	return home
}

// interface method set

// cache instances of hosts as they get used frequently
var hosts sync.Map

func New(name Name) *Host {
	parts := strings.SplitN(string(name), "@", 2)
	name = Name(parts[0])
	if len(parts) > 1 && parts[1] != string(LOCALHOST) {
		logError.Println("hosts on remote hosts not supported")
		return nil
	}

	r, ok := hosts.Load(name)
	if ok {
		rem, ok := r.(*Host)
		if ok {
			return rem
		}
	}

	// Bootstrap
	c := &Host{}
	c.Conf = viper.New()
	c.Name = name
	c.V().SetDefault("geneos", Geneos())
	c.Home = filepath.Join(c.V().GetString("geneos"), "hosts", string(c.Name))

	// fill this in directly as there is no config file to load
	if c.Name == LOCALHOST {
		c.GetOSReleaseEnv()
	}

	hosts.Store(name, c)
	return c
}

func (h *Host) V() *viper.Viper {
	return h.Conf
}

func (h *Host) Load() {
	if err := ReadConfig(h); err != nil {
		// logError.Println(err)
		return
	}
	h.ConfigLoaded = true
}

func (h *Host) Loaded() bool {
	if h == LOCAL || h == ALL {
		return true
	}
	return h.ConfigLoaded
}

func (h *Host) Unload() {
	hosts.Delete(h.Name)
	h.ConfigLoaded = false
}

func (host Name) String() string {
	return string(host)
}

func (h *Host) String() string {
	return string(h.Name)
}

func (h *Host) GetOSReleaseEnv() (err error) {
	osinfo := make(map[string]string)
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
		osinfo[key] = value
	}
	h.V().Set("osinfo", osinfo)
	return
}

func Get(remote Name) (r *Host) {
	switch remote {
	case LOCALHOST:
		return LOCAL
	case ALLHOSTS:
		return ALL
	default:
		i := New(remote)
		i.Load()
		return i
	}
}

func Match(remote Name) (r []*Host) {
	switch remote {
	case LOCALHOST:
		return []*Host{LOCAL}
	case ALLHOSTS:
		return AllHosts()
	default:
		i := New(remote)
		i.Load()
		return []*Host{i}
	}
}

// return an absolute path anchored in the root directory of the remote
// this can also be LOCAL
func (r *Host) GeneosJoinPath(paths ...string) string {
	return filepath.Join(append([]string{r.V().GetString("geneos")}, paths...)...)
}

func (r *Host) FullName(name string) string {
	if strings.Contains(name, "@") {
		return name
	}
	return name + "@" + r.String()
}

func AllHosts() (hosts []*Host) {
	hosts = []*Host{LOCAL}
	if utils.IsSuperuser() {
		return
	}

	for _, d := range FindHostDirs() {
		hosts = append(hosts, Get(Name(d)))
	}
	return
}
