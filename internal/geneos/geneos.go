package geneos

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
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
func Init(r *host.Host, ignoreExisting bool, args []string) (err error) {
	var root string
	var uid, gid int
	var username, homedir string
	var params []string

	if r != host.LOCAL && utils.IsSuperuser() {
		err = ErrNotSupported
		return
	}

	logDebug.Println("args:", args)

	if utils.IsSuperuser() {
		if len(args) == 0 {
			logError.Fatalln("init requires a USERNAME when run as root")
		}
		username = args[0]
		uid, gid, _, err = utils.GetIDs(username)

		if err != nil {
			logError.Fatalln("invalid user", username)
		}
		u, err := user.Lookup(username)
		homedir = u.HomeDir
		if err != nil {
			logError.Fatalln("user lookup failed")
		}
		if len(args) == 1 {
			// If user's home dir doesn't end in "geneos" then create a
			// directory "geneos" else use the home directory directly
			root = homedir
			if filepath.Base(homedir) != "geneos" {
				root = filepath.Join(homedir, "geneos")
			}
		} else {
			// must be an absolute path or relative to given user's home
			root = args[1]
			if !strings.HasPrefix(root, "/") {
				root = homedir
				if filepath.Base(homedir) != "geneos" {
					root = filepath.Join(homedir, root)
				}
			}
		}
	} else {
		if r == host.LOCAL {
			u, _ := user.Current()
			username = u.Username
			homedir = u.HomeDir
		} else {
			username = r.V().GetString("username")
			homedir = r.V().GetString("geneos")
		}
		logDebug.Println(len(args), args)
		switch len(args) {
		case 0: // default home + geneos
			root = homedir
			if filepath.Base(homedir) != "geneos" {
				root = filepath.Join(homedir, "geneos")
			}
		case 1: // home = abs path
			if !filepath.IsAbs(args[0]) {
				logError.Fatalln("Home directory must be absolute path:", args[0])
			}
			root = filepath.Clean(args[0])
		default:
			logError.Fatalln("too many args:", args, params)
		}
	}

	// dir must first not exist (or be empty) and then be createable
	//
	// maybe check that the entire list of registered directories are
	// either directories or do not exist
	if _, err := r.Stat(root); err != nil {
		if err = r.MkdirAll(root, 0775); err != nil {
			logError.Fatalln(err)
		}
	} else if !ignoreExisting {
		// check empty
		dirs, err := r.ReadDir(root)
		if err != nil {
			logError.Fatalln(err)
		}
		for _, entry := range dirs {
			if !strings.HasPrefix(entry.Name(), ".") {
				if r != host.LOCAL {
					logDebug.Println("remote directories exist, ending initialisation")
					return nil
				}
				logError.Fatalf("target directory %q exists and is not empty", root)
			}
		}
	}

	if r == host.LOCAL {
		viper.Set("geneos", root)
		viper.Set("defaultuser", username)

		if utils.IsSuperuser() {
			if err = host.LOCAL.WriteConfigFile(GlobalConfig, "root", 0664, viper.AllSettings()); err != nil {
				logError.Fatalln("cannot write global config", err)
			}

			// if everything else worked, remove any existing user config
			_ = r.Remove(filepath.Join(root, ".config", "geneos.json"))
		} else {
			userConfDir, err := os.UserConfigDir()
			if err != nil {
				logError.Fatalln(err)
			}
			userConfFile := filepath.Join(userConfDir, "geneos.json")

			if err = host.LOCAL.WriteConfigFile(userConfFile, username, 0664, viper.AllSettings()); err != nil {
				return err
			}
		}
	}

	// recreate host.LOCAL to load Geneos and others
	host.LOCAL.Unload()
	host.LOCAL = host.New(host.LOCALHOST)

	if utils.IsSuperuser() {
		if err = host.LOCAL.Chown(root, uid, gid); err != nil {
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
		err = filepath.WalkDir(root, func(path string, dir fs.DirEntry, err error) error {
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
