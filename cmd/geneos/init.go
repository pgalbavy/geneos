package main

import (
	"flag"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func init() {
	RegsiterCommand(Command{
		Name:        "init",
		Function:    commandInit,
		ParseFlags:  initFlag,
		ParseArgs:   nil,
		CommandLine: `geneos init [USERNAME] [DIRECTORY] [-T|-D|-A FILE|-S] [-n NAME] [-g FILE|URL] [-s FILE|URL] [-c CERTFILE] [-k KEYFILE] [VARS]`,
		Summary:     `Initialise a Geneos installation`,
		Description: `Initialise a Geneos installation by creating the directory hierarchy and
user configuration file, with the USERNAME and DIRECTORY if supplied.
DIRECTORY must be an absolute path and this is used to distinguish it
from USERNAME.

DIRECTORY defaults to ${HOME}/geneos for the selected user
unless the last compoonent of ${HOME} is 'geneos' in which case the
home directory is used. e.g. if the user is 'geneos' and the home
directory is '/opt/geneos' then that is used, but if it were a user
'itrs' which a home directory of '/home/itrs' then the directory
'home/itrs/geneos' would be used. This only applies when no DIRECTORY
is explicitly supplied.

When DIRECTORY is given it must be an absolute path and the parent
directory must be writable by the user - either running the command
or given as USERNAME.

DIRECTORY, whether explict or implied, must not exist or be empty of
all except "dot" files and directories.

When run with superuser privileges a USERNAME must be supplied and
only the configuration file for that user is created. e.g.:

	sudo geneos init geneos /opt/itrs

When USERNAME is supplied then the command must either be run with
superuser privileges or be run by the same user.

Any VARS provided are passed to the 'add' command called for
components created

FLAGS:

	-A LICENSE	Initialise a basic environment an import the give file as a license for licd
	-D		Initialise a Demo environment
	-T		Rebuild templates
	-S		Initialise a environment with one Self-Announcing Netprobe.
			If a signing certificate and key are provided then also
			create a certificate and connect with TLS. If a SAN
			template is provided (-s path) then use that to create
			the configuration. The default template uses the hostname
			to identify the SAN unless -n NAME is given.

	-n NAME		Use NAME as the default for instances and
			configurations instead of the hostname

	-c CERTFILE	Import the CERTFILE (which can be a URL) with an optional embedded
			private key. This also intialises the TLS environment and all
			new instances have certificates created for them.
	-k KEYFILE	Import the KEYFILE as a signing key. Overrides any embedded key in CERTFILE above

	-g TEMPLATE	Import a Gateway template file (local or URL) to replace of built-in default
	-s TEMPLATE	Import a San template file (local or URL) to replace the built-in default 

	The '-d', '-a' and '-S' flags are mutually exclusive.`,
	})

	initFlagSet = flag.NewFlagSet("init", flag.ExitOnError)
	initFlagSet.StringVar(&initFlags.All, "A", "", "Perform initialisation steps using provided license file and start environment")
	initFlagSet.BoolVar(&initFlags.Demo, "D", false, "Perform initialisation steps for a demo setup and start environment")
	initFlagSet.BoolVar(&initFlags.StartSAN, "S", false, "Create a SAN and start")
	initFlagSet.BoolVar(&initFlags.Templates, "T", false, "Overwrite/create templates from embedded (for version upgrades)")
	initFlagSet.StringVar(&initFlags.Name, "n", "", "Use the given name for instances and configurations instead of the hostname")
	initFlagSet.StringVar(&initFlags.SigningCert, "c", "", "signing certificate file with optional embedded private key")
	initFlagSet.StringVar(&initFlags.SigningKey, "k", "", "signing private key file")
	initFlagSet.StringVar(&initFlags.GatewayTmpl, "g", "", "A `gateway` template file")
	initFlagSet.StringVar(&initFlags.SanTmpl, "s", "", "A `san` template file")
	initFlagSet.BoolVar(&helpFlag, "h", false, helpUsage)
}

type initFlagsType struct {
	Demo, StartSAN, Templates bool
	Name                      string
	All                       string
	SigningCert, SigningKey   string
	GatewayTmpl, SanTmpl      string
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

	args = initFlag("init", args)

	// rewrite local templates and exit
	if initFlags.Templates {
		gatewayTemplates := rLOCAL.GeneosPath(Gateway.String(), "templates")
		rLOCAL.mkdirAll(gatewayTemplates, 0775)
		tmpl := GatewayTemplate
		if initFlags.GatewayTmpl != "" {
			tmpl = readSourceBytes(initFlags.GatewayTmpl)
		}
		if err := rLOCAL.writeFile(filepath.Join(gatewayTemplates, GatewayDefaultTemplate), tmpl, 0664); err != nil {
			log.Fatalln(err)
		}

		sanTemplates := rLOCAL.GeneosPath(San.String(), "templates")
		rLOCAL.mkdirAll(sanTemplates, 0775)
		tmpl = SanTemplate
		if initFlags.SanTmpl != "" {
			tmpl = readSourceBytes(initFlags.SanTmpl)
		}
		if err := rLOCAL.writeFile(filepath.Join(sanTemplates, SanDefaultTemplate), tmpl, 0664); err != nil {
			log.Fatalln(err)
		}

		return
	}

	// cannot pass both flags
	if initFlags.Demo && initFlags.All != "" {
		return ErrInvalidArgs
	}

	if err = rLOCAL.initGeneos(args); err != nil {
		log.Fatalln(err)
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

	// re-run defaultArgs?
	_, args, params = defaultArgs(Command{
		Name:          "init",
		Wildcard:      false,
		ComponentOnly: false,
		ParseFlags:    initFlag,
	}, args)

	logDebug.Println("args, params:", args, params)

	if len(params) > 0 {
		if err = initFlagSet.Parse(params); err != nil {
			log.Fatalln(err)
		}

		params = initFlagSet.Args()
	}

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
			homedir = r.ITRSHome
		}
		switch len(args) {
		case 0: // default home + geneos
			dir = homedir
			if filepath.Base(homedir) != "geneos" {
				dir = filepath.Join(homedir, "geneos")
			}
		case 1: // home = abs path
			if !filepath.IsAbs(args[0]) {
				log.Fatalln("Home directory must be absolute path:", args[0])
			}
			dir = filepath.Clean(args[0])
		default:
			logError.Fatalln("too many args:", args, params)
		}
	}

	// dir must first not exist (or be empty) and then be createable
	if _, err := r.statFile(dir); err == nil {
		// check empty
		dirs, err := r.readDir(dir)
		if err != nil {
			logError.Fatalln(err)
		}
		for _, entry := range dirs {
			if !strings.HasPrefix(entry.Name(), ".") {
				logError.Fatalf("target directory %q exists and is not empty", dir)
			}
		}
	} else {
		// need to create out own, chown base directory only
		if err = r.mkdirAll(dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}

	if superuser {
		if err = rLOCAL.chown(dir, uid, gid); err != nil {
			logError.Fatalln(err)
		}
	}

	// create directories
	for _, d := range initDirs {
		dir := filepath.Join(dir, d)
		if err = r.mkdirAll(dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}

	if superuser {
		err = filepath.WalkDir(dir, func(path string, dir fs.DirEntry, err error) error {
			if err == nil {
				err = rLOCAL.chown(path, uid, gid)
			}
			return err
		})
	}

	if r == rLOCAL {
		c := make(GlobalSettings)
		c["ITRSHome"] = dir
		c["DefaultUser"] = username

		if superuser {
			if err = rLOCAL.writeConfigFile(globalConfig, "root", c); err != nil {
				logError.Fatalln("cannot write global config", err)
			}

			// if everything else worked, remove any existing user config
			_ = r.removeFile(filepath.Join(dir, ".config", "geneos.json"))
		} else {
			userConfDir, err := os.UserConfigDir()
			if err != nil {
				log.Fatalln(err)
			}
			userConfFile := filepath.Join(userConfDir, "geneos.json")

			if err = rLOCAL.writeConfigFile(userConfFile, username, c); err != nil {
				return err
			}
		}
	}

	// now reload config, after init
	loadSysConfig()

	// also recreate rLOCAL to load ITRSHome and others
	delete(remotes, LOCAL)
	rLOCAL = NewRemote(string(LOCAL)).(*Remotes)

	for _, c := range components {
		if c.Initialise != nil {
			c.Initialise(rLOCAL)
		}
	}

	if initFlags.GatewayTmpl != "" {
		tmpl := readSourceBytes(initFlags.GatewayTmpl)
		if err := rLOCAL.writeFile(rLOCAL.GeneosPath(Gateway.String(), "templates", GatewayDefaultTemplate), tmpl, 0664); err != nil {
			log.Fatalln(err)
		}
	}

	if initFlags.SanTmpl != "" {
		tmpl := readSourceBytes(initFlags.SanTmpl)
		if err := rLOCAL.writeFile(rLOCAL.GeneosPath(San.String(), "templates", SanDefaultTemplate), tmpl, 0664); err != nil {
			log.Fatalln(err)
		}
	}

	// both options can import arbitrary PEM files, fix this
	if initFlags.SigningCert != "" {
		TLSImport(initFlags.SigningCert)
	}

	if initFlags.SigningKey != "" {
		TLSImport(initFlags.SigningKey)
	}

	e := []string{}
	rem := []string{"@" + r.InstanceName}

	// create a demo environment
	if initFlags.Demo {
		g := []string{"Demo Gateway@" + r.InstanceName}
		n := []string{"localhost@" + r.InstanceName}
		w := []string{"demo@" + r.InstanceName}
		commandDownload(None, e, e)
		commandAdd(Gateway, g, params)
		commandSet(Gateway, g, []string{"GateOpts=-demo"})
		commandAdd(Netprobe, n, params)
		commandAdd(Webserver, w, params)
		// call defaultArgs() on an empty list to populate for loopCommand()
		ct, args, params := defaultArgs(commands["start"], rem)
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
		g := []string{initFlags.Name}
		n := []string{"localhost@" + r.InstanceName}
		commandDownload(None, e, e)
		commandAdd(Licd, g, params)
		commandImport(Licd, g, []string{"geneos.lic=" + initFlags.All})
		commandAdd(Gateway, g, params)
		commandAdd(Netprobe, n, params)
		commandAdd(Webserver, g, params)
		// call defaultArgs() on an empty list to populate for loopCommand()
		ct, args, params := defaultArgs(commands["start"], rem)
		commandStart(ct, args, params)
		commandPS(ct, args, params)
		return nil
	}

	return
}

func initFlag(command string, args []string) []string {
	initFlagSet.Parse(args)
	if initFlags.Name != "" && !validInstanceName(initFlags.Name) {
		log.Fatalln(initFlags.Name, "is not a valid name")
	}
	checkHelpFlag(command)
	return initFlagSet.Args()
}
