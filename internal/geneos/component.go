package geneos

import (
	"path/filepath"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/pkg/logger"
)

// definitions and access methods for the generic component types

// type ComponentType string

type DownloadBases struct {
	Resources string
	Nexus     string
}

type Component struct {
	Initialise       func(*host.Host, *Component)
	New              func(string) Instance
	Name             string
	RelatedTypes     []*Component
	ComponentMatches []string
	RealComponent    bool
	DownloadBase     DownloadBases
	PortRange        string
	CleanList        string
	PurgeList        string
	Aliases          map[string]string
	Defaults         []string // ordered list of key=value pairs
	GlobalSettings   map[string]string
	Directories      []string
}

type Instance interface {
	// getters and setters
	Name() string
	Home() string
	Type() *Component
	Host() *host.Host
	Prefix() string
	String() string

	// config
	Load() error
	Unload() error
	Loaded() bool
	V() *viper.Viper
	SetConf(*viper.Viper)

	// actions
	Add(string, string, uint16) error
	Command() ([]string, []string)
	Reload(params []string) (err error)
	Rebuild(bool) error
}

var (
	log      = logger.Log
	logDebug = logger.Debug
	logError = logger.Error
)

var Root Component = Component{
	Name:             "none",
	RelatedTypes:     nil,
	ComponentMatches: []string{"all", "any"},
	RealComponent:    false,
	DownloadBase:     DownloadBases{Resources: "", Nexus: ""},
	GlobalSettings: map[string]string{
		// Root directory for all operations
		"geneos": "",

		// Root URL for all downloads of software archives
		"download.url": "https://resources.itrsgroup.com/download/latest/",

		// Username to start components if not explicitly defined
		// and we are running with elevated privileges
		//
		// When running as a normal user this is unused and
		// we simply test a defined user against the running user
		//
		// default is owner of Geneos
		"defaultuser": "",

		// Path List seperated additions to the reserved names list, over and above
		// any words matched by ParseComponentName()
		"reservednames": "",

		"privatekeys": "id_rsa,id_ecdsa,id_ecdsa_sk,id_ed25519,id_ed25519_sk,id_dsa",
	},
	Directories: []string{
		"packages/downloads",
		"hosts",
	},
}

func init() {
	RegisterComponent(&Root, nil)
}

type ComponentsMap map[string]*Component

// slice of registered component types for indirect calls
// this should actually become an Interface
var components ComponentsMap = make(ComponentsMap)

func AllComponents() (cts []*Component) {
	for _, c := range components {
		cts = append(cts, c)
	}
	return
}

// currently supported real component types, for looping
// (go doesn't allow const slices, a function is the workaround)
func RealComponents() (cts []*Component) {
	for _, c := range components {
		if c.RealComponent {
			cts = append(cts, c)
		}
	}
	return
}

// register a component type
//
// the factory function is an arg to disguise init cycles
// when you declare it in the struct in the caller
func RegisterComponent(ct *Component, n func(string) Instance) {
	ct.New = n
	components[ct.Name] = ct
	ct.RegisterDirs(ct.Directories)
	for k, v := range ct.GlobalSettings {
		viper.SetDefault(k, v)
	}
}

var initDirs map[string][]string = make(map[string][]string)

// register directories that need to be created in the
// root of the install (by init)
func (ct Component) RegisterDirs(dirs []string) {
	initDirs[ct.Name] = dirs
}

func (ct Component) String() (name string) {
	return ct.Name
}

// return the component type by iterating over all the
// names registered by components. case sensitive.
func ParseComponentName(component string) *Component {
	for _, v := range components {
		for _, m := range v.ComponentMatches {
			if m == component {
				return v
			}
		}
	}
	return nil
}

// create any missing component registered directories
func MakeComponentDirs(h *host.Host, ct *Component) (err error) {
	var name string
	if h == host.ALL {
		logError.Fatalln("called with all hosts")
	}
	if ct == nil {
		name = "none"
	} else {
		name = ct.Name
	}
	for _, d := range initDirs[name] {
		dir := filepath.Join(h.GetString("geneos"), d)
		logDebug.Println("mkdirall", dir)
		if err = h.MkdirAll(dir, 0775); err != nil {
			return
		}
	}
	return
}

// Return the base directory for a Component
// ct cannot be None
func (ct *Component) ComponentDir(h *host.Host) string {
	p := h.GeneosJoinPath(ct.String(), ct.String()+"s")
	return p
}
