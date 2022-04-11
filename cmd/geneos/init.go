package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func init() {
	RegisterCommand(Command{
		Name:          "init",
		Function:      commandInit,
		ParseFlags:    initFlag,
		ParseArgs:     nil,
		Wildcard:      false,
		ComponentOnly: false,
		CommandLine:   `geneos init [-A FILE|URL|-D|-S|-T] [-n NAME] [-g FILE|URL] [-s FILE|URL] [-c CERTFILE] [-k KEYFILE] [USERNAME] [DIRECTORY] [PARAMS]`,
		Summary:       `Initialise a Geneos installation`,
		Description: `Initialise a Geneos installation by creating the directory hierarchy and
user configuration file, with the USERNAME and DIRECTORY if supplied.
DIRECTORY must be an absolute path and this is used to distinguish it
from USERNAME.

DIRECTORY defaults to ${HOME}/geneos for the selected user
unless the last component of ${HOME} is 'geneos' in which case the
home directory is used. e.g. if the user is 'geneos' and the home
directory is '/opt/geneos' then that is used, but if it were a user
'itrs' which a home directory of '/home/itrs' then the directory
'home/itrs/geneos' would be used. This only applies when no DIRECTORY
is explicitly supplied.

When DIRECTORY is given it must be an absolute path and the parent
directory must be writable by the user - either running the command
or given as USERNAME.

DIRECTORY, whether explicit or implied, must not exist or be empty of
all except "dot" files and directories.

When run with superuser privileges a USERNAME must be supplied and
only the configuration file for that user is created. e.g.:

	sudo geneos init geneos /opt/itrs

When USERNAME is supplied then the command must either be run with
superuser privileges or be run by the same user.

Any PARAMS provided are passed to the 'add' command called for
components created

FLAGS:

	-A LICENSE	Initialise a basic environment an import the give file as a license for licd
	-C		Create default certificates for TLS support
	-D		Initialise a Demo environment
	-S		Initialise a environment with one Self-Announcing Netprobe.
		If a signing certificate and key are provided then also
		create a certificate and connect with TLS. If a SAN
		template is provided (-s path) then use that to create
		the configuration. The default template uses the hostname
		to identify the SAN unless -n NAME is given.
	-T		Rebuild templates

	-n NAME		Use NAME as the default for instances and
			configurations instead of the hostname

	-g TEMPLATE	Import a Gateway template file (local or URL) to replace of built-in default
	-s TEMPLATE	Import a San template file (local or URL) to replace the built-in default 

	-c CERTFILE	Import the CERTFILE (which can be a URL) with an optional embedded
			private key. This also intialises the TLS environment and all
			new instances have certificates created for them.
	-k KEYFILE	Import the KEYFILE as a signing key. Overrides any embedded key in CERTFILE above

	The '-A', '-D', '-S' and '-T' flags are mutually exclusive.`,
	})

	initFlagSet = flag.NewFlagSet("init", flag.ExitOnError)
	initFlagSet.StringVar(&initFlags.All, "A", "",
		"Perform initialisation steps using provided license file and start environment")
	initFlagSet.BoolVar(&initFlags.Certs, "C", false, "Create default certificates for TLS support")
	initFlagSet.BoolVar(&initFlags.Demo, "D", false,
		"Perform initialisation steps for a demo setup and start environment")
	initFlagSet.BoolVar(&initFlags.StartSAN, "S", false,
		"Create a SAN and start")
	initFlagSet.BoolVar(&initFlags.Templates, "T", false,
		"Overwrite/create templates from embedded (for version upgrades)")

	initFlagSet.StringVar(&initFlags.Name, "n", "",
		"Use the given name for instances and configurations instead of the hostname")

	initFlagSet.StringVar(&initFlags.SigningCert, "c", "",
		"signing certificate file with optional embedded private key")
	initFlagSet.StringVar(&initFlags.SigningKey, "k", "",
		"signing private key file")

	initFlagSet.StringVar(&initFlags.GatewayTmpl, "g", "",
		"A `gateway` template file")
	initFlagSet.StringVar(&initFlags.SanTmpl, "s", "",
		"A `san` template file")
	initFlagSet.BoolVar(&helpFlag, "h", false, helpUsage)
}

type initFlagsType struct {
	Certs, Demo, StartSAN, Templates bool
	Name                             string
	All                              string
	SigningCert, SigningKey          string
	GatewayTmpl, SanTmpl             string
}

var initFlagSet, deleteFlags *flag.FlagSet
var initFlags initFlagsType

//
// initialise a geneos installation
//
// if no directory given and not running as root and the last component of the user's
// home direcvtory is NOT "geneos" then create a directory "geneos", else
//
// XXX Call any registered initialiser funcs from components
//
func commandInit(ct Component, args []string, params []string) (err error) {
	// none of the arguments can be a reserved type
	if ct != None {
		return ErrInvalidArgs
	}

	// as we don't use parseArgs() we have to consume flags here
	args = initFlag("init", args)

	// rewrite local templates and exit
	if initFlags.Templates {
		gatewayTemplates := rLOCAL.GeneosPath(Gateway.String(), "templates")
		rLOCAL.MkdirAll(gatewayTemplates, 0775)
		tmpl := GatewayTemplate
		if initFlags.GatewayTmpl != "" {
			if tmpl, err = readLocalFileOrURL(initFlags.GatewayTmpl); err != nil {
				return
			}
		}
		if err := rLOCAL.WriteFile(filepath.Join(gatewayTemplates, GatewayDefaultTemplate), tmpl, 0664); err != nil {
			logError.Fatalln(err)
		}

		tmpl = InstanceTemplate
		if err := rLOCAL.WriteFile(filepath.Join(gatewayTemplates, GatewayInstanceTemplate), tmpl, 0664); err != nil {
			logError.Fatalln(err)
		}

		sanTemplates := rLOCAL.GeneosPath(San.String(), "templates")
		rLOCAL.MkdirAll(sanTemplates, 0775)
		tmpl = SanTemplate
		if initFlags.SanTmpl != "" {
			if tmpl, err = readLocalFileOrURL(initFlags.SanTmpl); err != nil {
				return
			}
		}
		if err := rLOCAL.WriteFile(filepath.Join(sanTemplates, SanDefaultTemplate), tmpl, 0664); err != nil {
			logError.Fatalln(err)
		}

		return
	}

	flagcount := 0
	for _, b := range []bool{initFlags.Demo, initFlags.Templates, initFlags.StartSAN} {
		if b {
			flagcount++
		}
	}

	if initFlags.All != "" {
		flagcount++
	}

	if flagcount > 1 {
		return fmt.Errorf("%w: Only one of -A, -D, -S or -T can be given", ErrInvalidArgs)
	}

	if err = rLOCAL.initGeneos(args); err != nil {
		logError.Fatalln(err)
	}

	return
}

func (r *Remotes) initGeneos(args []string) (err error) {
	var dir string
	var uid, gid int
	var username, homedir string
	var params []string

	if r != rLOCAL && superuser {
		err = ErrNotSupported
		return
	}

	// split params into their own list
	_, args, params = parseArgs(Command{
		Name:          "init",
		Wildcard:      false,
		ComponentOnly: false,
		ParseFlags:    initFlag,
	}, args)

	logDebug.Println("args:", args)

	if superuser {
		if len(args) == 0 {
			logError.Fatalln("init requires a USERNAME when run as root")
		}
		username = args[0]
		uid, gid, _, err = getUser(username)

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
			dir = homedir
			if filepath.Base(homedir) != "geneos" {
				dir = filepath.Join(homedir, "geneos")
			}
		} else {
			// must be an absolute path or relative to given user's home
			dir = args[1]
			if !strings.HasPrefix(dir, "/") {
				dir = homedir
				if filepath.Base(homedir) != "geneos" {
					dir = filepath.Join(homedir, dir)
				}
			}
		}
	} else {
		if r == rLOCAL {
			u, _ := user.Current()
			username = u.Username
			homedir = u.HomeDir
		} else {
			username = r.Username
			homedir = r.Geneos
		}
		switch len(args) {
		case 0: // default home + geneos
			dir = homedir
			if filepath.Base(homedir) != "geneos" {
				dir = filepath.Join(homedir, "geneos")
			}
		case 1: // home = abs path
			if !filepath.IsAbs(args[0]) {
				logError.Fatalln("Home directory must be absolute path:", args[0])
			}
			dir = filepath.Clean(args[0])
		default:
			logError.Fatalln("too many args:", args, params)
		}
	}

	// dir must first not exist (or be empty) and then be createable
	// XXX have an ignore flag?
	// maybe check that the entire list of registered directories are
	// either directories or do not exist
	if _, err := r.Stat(dir); err == nil {
		// check empty
		dirs, err := r.ReadDir(dir)
		if err != nil {
			logError.Fatalln(err)
		}
		for _, entry := range dirs {
			if !strings.HasPrefix(entry.Name(), ".") {
				if r != rLOCAL {
					logDebug.Println("remote directories exist, ending initialisation")
					return nil
				}
				logError.Fatalf("target directory %q exists and is not empty", dir)
			}
		}
	} else {
		// need to create out own, chown base directory only
		if err = r.MkdirAll(dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}

	if r == rLOCAL {
		c := make(GlobalSettings)
		c["Geneos"] = dir
		c["DefaultUser"] = username

		if superuser {
			if err = rLOCAL.writeConfigFile(globalConfig, "root", 0664, c); err != nil {
				logError.Fatalln("cannot write global config", err)
			}

			// if everything else worked, remove any existing user config
			_ = r.Remove(filepath.Join(dir, ".config", "geneos.json"))
		} else {
			userConfDir, err := os.UserConfigDir()
			if err != nil {
				logError.Fatalln(err)
			}
			userConfFile := filepath.Join(userConfDir, "geneos.json")

			if err = rLOCAL.writeConfigFile(userConfFile, username, 0664, c); err != nil {
				return err
			}
		}
	}

	// now reload config, after init
	loadSysConfig()

	// also recreate rLOCAL to load Geneos and others
	rLOCAL.Unload()
	rLOCAL = NewRemote(string(LOCAL)).(*Remotes)

	if superuser {
		if err = rLOCAL.Chown(dir, uid, gid); err != nil {
			logError.Fatalln(err)
		}
	}

	if err = None.makeComponentDirs(rLOCAL); err != nil {
		return
	}

	if superuser {
		err = filepath.WalkDir(dir, func(path string, dir fs.DirEntry, err error) error {
			if err == nil {
				err = rLOCAL.Chown(path, uid, gid)
			}
			return err
		})
	}

	for _, c := range components {
		if c.Initialise != nil {
			c.Initialise(rLOCAL)
		}
	}

	if initFlags.GatewayTmpl != "" {
		var tmpl []byte
		if tmpl, err = readLocalFileOrURL(initFlags.GatewayTmpl); err != nil {
			return
		}
		if err := rLOCAL.WriteFile(rLOCAL.GeneosPath(Gateway.String(), "templates", GatewayDefaultTemplate), tmpl, 0664); err != nil {
			logError.Fatalln(err)
		}
	}

	if initFlags.SanTmpl != "" {
		var tmpl []byte
		if tmpl, err = readLocalFileOrURL(initFlags.SanTmpl); err != nil {
			return
		}
		if err = rLOCAL.WriteFile(rLOCAL.GeneosPath(San.String(), "templates", SanDefaultTemplate), tmpl, 0664); err != nil {
			return
		}
	}

	if initFlags.Certs {
		TLSInit()
	} else {
		// both options can import arbitrary PEM files, fix this
		if initFlags.SigningCert != "" {
			TLSImport(initFlags.SigningCert)
		}

		if initFlags.SigningKey != "" {
			TLSImport(initFlags.SigningKey)
		}
	}
	e := []string{}
	rem := []string{"@" + r.InstanceName}

	// create a demo environment
	if initFlags.Demo {
		g := []string{"Demo Gateway@" + r.InstanceName}
		n := []string{"localhost@" + r.InstanceName}
		w := []string{"demo@" + r.InstanceName}
		commandInstall(Gateway, e, e)
		commandAdd(Gateway, g, params)
		commandSet(Gateway, g, []string{"GateOpts=-demo"})
		commandInstall(San, e, e)
		commandAdd(San, n, []string{"Gateways=localhost"})
		commandInstall(Webserver, e, e)
		commandAdd(Webserver, w, params)
		// call parseArgs() on an empty list to populate for loopCommand()
		ct, args, params := parseArgs(commands["start"], rem)
		commandStart(ct, args, params)
		commandPS(ct, args, params)
		return
	}

	if initFlags.StartSAN {
		var sanname string
		var s []string

		if initFlags.Name != "" {
			sanname = initFlags.Name
		} else {
			sanname, _ = os.Hostname()
		}
		if r != rLOCAL {
			sanname = sanname + "@" + r.InstanceName
		}
		s = []string{sanname}
		// Add will also install the right package
		commandAdd(San, s, params)
		return nil
	}

	// create a basic environment with license file
	if initFlags.All != "" {
		if initFlags.Name == "" {
			initFlags.Name, err = os.Hostname()
			if err != nil {
				return err
			}
		}
		name := []string{initFlags.Name}
		localhost := []string{"localhost@" + r.InstanceName}
		commandInstall(Licd, e, e)
		commandAdd(Licd, name, params)
		commandImport(Licd, name, []string{"geneos.lic=" + initFlags.All})
		commandInstall(Gateway, e, e)
		commandAdd(Gateway, name, params)
		commandInstall(Netprobe, e, e)
		commandAdd(Netprobe, localhost, params)
		commandInstall(Webserver, e, e)
		commandAdd(Webserver, name, params)
		// call parseArgs() on an empty list to populate for loopCommand()
		ct, args, params := parseArgs(commands["start"], rem)
		commandStart(ct, args, params)
		commandPS(ct, args, params)
		return nil
	}

	return
}

func initFlag(command string, args []string) []string {
	initFlagSet.Parse(args)
	if initFlags.Name != "" && !validInstanceName(initFlags.Name) {
		logError.Fatalln(initFlags.Name, "is not a valid name")
	}
	checkHelpFlag(command)
	return initFlagSet.Args()
}
