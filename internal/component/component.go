package component

import (
	"path/filepath"

	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/pkg/logger"
)

// definitions and access methods for the generic component types

type ComponentType string

const (
	None    ComponentType = ""
	Unknown ComponentType = "unknown"
)

type Components struct {
	// function to call from 'init' and 'add remote' commands to set-up environment
	// arg is the name of the remote
	Initialise func(*host.Host)

	// function to create a new instance of component
	New func(string) interface{}

	ComponentType    ComponentType
	RelatedTypes     []ComponentType
	ComponentMatches []string
	RealComponent    bool
	DownloadBase     string
	PortRange        string
	CleanList        string
	PurgeList        string
	DefaultSettings  map[string]string
}

var (
	log      = logger.Log
	logDebug = logger.Debug
	logError = logger.Error
)

func init() {
	RegisterComponent(Components{
		ComponentType:    None,
		RelatedTypes:     nil,
		ComponentMatches: []string{"", "all", "any"},
		RealComponent:    false,
		DownloadBase:     "",
		DefaultSettings: map[string]string{
			// Root directory for all operations
			"Geneos": "",

			// Root URL for all downloads of software archives
			"DownloadURL": "https://resources.itrsgroup.com/download/latest/",

			// Username to start components if not explicitly defined
			// and we are running with elevated privileges
			//
			// When running as a normal user this is unused and
			// we simply test a defined user against the running user
			//
			// default is owner of Geneos
			"DefaultUser": "",

			// Path List seperated additions to the reserved names list, over and above
			// any words matched by ParseComponentName()
			"ReservedNames": "",

			"PrivateKeys": "id_rsa,id_ecdsa,id_ecdsa_sk,id_ed25519,id_ed25519_sk,id_dsa",
		},
	})
}

type ComponentsMap map[ComponentType]Components

// slice of registered component types for indirect calls
// this should actually become an Interface
var components ComponentsMap = make(ComponentsMap)

// currently supported real component types, for looping
// (go doesn't allow const slices, a function is the workaround)
// not including Remote - this is special
func RealComponents() (cts []ComponentType) {
	for ct, c := range components {
		if c.RealComponent {
			cts = append(cts, ct)
		}
	}
	return
}

// register a component type
func RegisterComponent(c Components) {
	components[c.ComponentType] = c
}

var initDirs map[ComponentType][]string = make(map[ComponentType][]string)

// register directories that need to be created in the
// root of the install (by init)
func (ct ComponentType) RegisterDirs(dirs []string) {
	initDirs[ct] = dirs
}

func (ct ComponentType) String() (name string) {
	return string(ct)
}

// return the component type by iterating over all the
// names registered by components. case sensitive.
func ParseComponentName(component string) ComponentType {
	for ct, v := range components {
		for _, m := range v.ComponentMatches {
			if m == component {
				return ct
			}
		}
	}
	return Unknown
}

// create any missing component registered directories
func MakeComponentDirs(h *host.Host, ct ComponentType) (err error) {
	if h == host.ALL {
		logError.Fatalln("called with all hosts")
	}
	for _, d := range initDirs[ct] {
		dir := filepath.Join(h.Geneos(), d)
		if err = h.MkdirAll(dir, 0775); err != nil {
			return
		}
	}
	return
}

// Return the base directory for a Component
// ct cannot be None
func (ct ComponentType) ComponentDir(r *host.Host) string {
	if ct == None {
		logError.Fatalln("must supply a component type")
	}
	return r.GeneosPath(ct.String(), ct.String()+"s")
}
