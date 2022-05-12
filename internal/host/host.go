package host

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const UserHostFile = "geneos-hosts.json"
const LOCALHOST = "localhost"
const ALLHOSTS = "all"

var LOCAL, ALL *Host

type Host struct {
	*viper.Viper
}

// private parent of all hosts
var hosts *viper.Viper

// this is called from cmd root
func Init() {
	LOCAL = New(LOCALHOST)
	ALL = New(ALLHOSTS)
	ReadConfigFile()
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

// XXX new needs the top level viper and passes back a Sub()
func New(name string) (c *Host) {
	switch name {
	case LOCALHOST:
		if LOCAL != nil {
			return LOCAL
		}
		c = &Host{viper.New()}
		c.Set("name", LOCALHOST)
		c.GetOSReleaseEnv()
	case ALLHOSTS:
		if ALL != nil {
			return ALL
		}
		c = &Host{viper.New()}
		c.Set("name", ALLHOSTS)
	default:
		// grab the existing one
		h := hosts.Sub(name)
		if h != nil {
			return &Host{h}
		}
		// or bootstrap, but NOT save a new one
		c = &Host{viper.New()}
		c.Set("name", name)
	}

	c.Set("geneos", Geneos())

	return
}

func Add(host *Host) {
	hosts.Set(host.String(), host.AllSettings())
}

// delete a host from the host list
// this is done by setting the host setting to an empty string as viper
// does not support setting a nil or unsetting a setting. this is then picked
// up in the write config file function and skipped
func Delete(host *Host) {
	hosts.Set(host.String(), "")
}

func (h *Host) Exists() bool {
	if h == LOCAL || h == ALL {
		return true
	}

	if !hosts.IsSet(h.String()) {
		return false
	}

	// work with Delete() above
	switch hosts.Get(h.String()).(type) {
	case string:
		return false
	default:
		return true
	}
}

func (h *Host) String() string {
	if h.IsSet("name") {
		return h.GetString("name")
	}
	return "unknown"
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
	h.Set("osinfo", osinfo)
	return
}

// returns a slice of all matching Hosts. used mainly for range loops
// where the host could be specific or 'all'
func Match(host string) (r []*Host) {
	switch host {
	case ALLHOSTS:
		return AllHosts()
	default:
		return []*Host{New(host)}
	}
}

// return an absolute path anchored in the root directory of the remote host
// this can also be LOCAL
func (h *Host) GeneosJoinPath(paths ...string) string {
	if h == nil {
		logError.Fatalln("host is nil")
	}

	return filepath.Join(append([]string{h.GetString("geneos")}, paths...)...)
}

func (h *Host) FullName(name string) string {
	if strings.Contains(name, "@") {
		return name
	}
	return name + "@" + h.String()
}

func AllHosts() (hs []*Host) {
	hs = []*Host{LOCAL}

	for k := range hosts.AllSettings() {
		hs = append(hs, New(k))
	}
	return
}

func ReadConfigFile() {
	hosts = viper.New()

	h := viper.New()
	h.SetConfigFile(UserHostsFilePath())
	h.ReadInConfig()
	if h.InConfig("hosts") {
		hosts = h.Sub("hosts")
	}
}

func WriteConfigFile() error {
	n := viper.New()
	for h, v := range hosts.AllSettings() {
		switch v.(type) {
		case string:
			// do nothing
			break
		default:
			n.Set("hosts."+h, v)
		}
	}
	return n.WriteConfigAs(UserHostsFilePath())
}

func UserHostsFilePath() string {
	userConfDir, err := os.UserConfigDir()
	if err != nil {
		logError.Fatalln(err)
	}
	return filepath.Join(userConfDir, UserHostFile)
}
