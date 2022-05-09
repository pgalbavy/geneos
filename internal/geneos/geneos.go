package geneos

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/utils"
)

var (
	ErrInvalidArgs  error = errors.New("invalid arguments")
	ErrNotSupported error = errors.New("not supported")
	ErrDisabled     error = errors.New("disabled")
)

const RootCAFile = "rootCA"
const SigningCertFile = "geneos"
const DisableExtension = "disabled"
const GlobalConfig = "/etc/geneos/geneos.json"

// initialise a Geneos environment.
//
// creates a directory hierarchy and calls the initialisation
// functions for each component, for example to create templates
//
// if the directory is not empty and 'noEmptyOK' is false then
// nothing is changed
func Init(r *host.Host, options ...GeneosOptions) (err error) {
	// var homedir string
	var uid, gid int

	// var params []string

	if r != host.LOCAL && utils.IsSuperuser() {
		err = ErrNotSupported
		return
	}

	opts := doOptions(options...)
	if opts.homedir == "" {
		// default or error
	}

	// dir must first not exist (or be empty) and then be createable
	//
	// maybe check that the entire list of registered directories are
	// either directories or do not exist
	if _, err := r.Stat(opts.homedir); err != nil {
		if err = r.MkdirAll(opts.homedir, 0775); err != nil {
			logError.Fatalln(err)
		}
	} else if !opts.overwrite {
		// check empty
		dirs, err := r.ReadDir(opts.homedir)
		if err != nil {
			logError.Fatalln(err)
		}
		for _, entry := range dirs {
			if !strings.HasPrefix(entry.Name(), ".") {
				if r != host.LOCAL {
					logDebug.Println("remote directories exist, exiting init")
					return nil
				}
				logError.Fatalf("target directory %q exists and is not empty", opts.homedir)
			}
		}
	}

	if r == host.LOCAL {
		viper.Set("geneos", opts.homedir)
		viper.Set("defaultuser", opts.username)

		if utils.IsSuperuser() {
			if err = host.LOCAL.WriteConfigFile(GlobalConfig, "root", 0664, viper.AllSettings()); err != nil {
				logError.Fatalln("cannot write global config", err)
			}

			// if everything else worked, remove any existing user config - why?
			// _ = r.Remove(filepath.Join(g.homedir, ".config", "geneos.json"))
		} else {
			userConfDir, err := os.UserConfigDir()
			if err != nil {
				logError.Fatalln(err)
			}
			userConfFile := filepath.Join(userConfDir, "geneos.json")

			if err = host.LOCAL.WriteConfigFile(userConfFile, opts.username, 0664, viper.AllSettings()); err != nil {
				return err
			}
		}
	}

	// recreate host.LOCAL to load Geneos and others
	host.LOCAL.Unload()
	host.LOCAL = host.New(host.LOCALHOST)

	if utils.IsSuperuser() {
		uid, gid, _, err = utils.GetIDs(opts.username)
		if err != nil {
			// do something
		}
		if err = host.LOCAL.Chown(opts.homedir, uid, gid); err != nil {
			logError.Fatalln(err)
		}
	}

	// it's not an error to try to re-create existing dirs
	if err = MakeComponentDirs(host.LOCAL, nil); err != nil {
		return
	}

	// if we've created directory paths as root, go through and change
	// ownership to the tree
	if utils.IsSuperuser() {
		err = filepath.WalkDir(opts.homedir, func(path string, dir fs.DirEntry, err error) error {
			if err == nil {
				err = host.LOCAL.Chown(path, uid, gid)
			}
			return err
		})
	}

	for _, c := range AllComponents() {
		if c.Initialise != nil {
			c.Initialise(host.LOCAL, c)
		}
	}

	return
}
