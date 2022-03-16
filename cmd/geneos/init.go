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
	commands["init"] = Command{
		Function:    commandInit,
		ParseFlags:  initFlag,
		ParseArgs:   nil,
		CommandLine: `geneos init [-d] [-a FILE] [-S] [-n NAME] [-g FILE|URL] [-s FILE|URL] [-c CERTFILE] [-k KEYFILE] [USERNAME] [DIRECTORY]`,
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

FLAGS:

	-d	Initialise a Demo environment
	-a LICENSE	Initialise a basic environment an import the give file as a license for licd
	-S gateway1[:port1][,gateway2[:port2]...]	Initialise a environment with one Self-Announcing Netprobe connecting to one or more gateways with optional port values. If a signing certificate and key are provided then create a cert and connect with TLS. If a SAN template is provided (-s below) then use that to create the configuration. The default template uses the hostname to identify the SAN.
	-n NAME	use NAME as the default for instances and configurations instead of the hostname
	-c CERTFILE	Import the CERTFILE as a signing certificate with an optional embedded private key. This also intialises the TLS environment and all instances have certificates created for them
	-k KEYFILE	Import the KEYFILE as a signing key. Overrides any embedded key in CERTFILE above

	-g TEMPLATE Import a Gateway template file (local or URL) to replace of built-in default
	-s TEMPLATE	Import a San template file (local or URL) to replace the built-in default 

	The '-d' and '-a' flags are mutually exclusive.
`}

	initFlagSet = flag.NewFlagSet("init", flag.ExitOnError)
	initFlagSet.BoolVar(&initFlags.Demo, "d", false, "Perform initialisation steps for a demo setup and start environment")
	initFlagSet.BoolVar(&initFlags.Templates, "t", false, "Overwrite/create templates from embedded (for version upgrades)")
	initFlagSet.StringVar(&initFlags.All, "a", "", "Perform initialisation steps using provided license file and start environment")
	initFlagSet.BoolVar(&initFlags.StartSAN, "S", false, "Create a SAN and start")
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

	// rewrite local templates
	if initFlags.Templates {
		gatewayTemplates := GeneosPath(LOCAL, Gateway.String(), "templates")
		mkdirAll(LOCAL, gatewayTemplates, 0775)
		tmpl := GatewayTemplate
		if initFlags.GatewayTmpl != "" {
			tmpl = readSourceBytes(initFlags.GatewayTmpl)
		}
		if err := writeFile(LOCAL, filepath.Join(gatewayTemplates, GatewayDefaultTemplate), tmpl, 0664); err != nil {
			log.Fatalln(err)
		}

		sanTemplates := GeneosPath(LOCAL, San.String(), "templates")
		mkdirAll(LOCAL, sanTemplates, 0775)
		tmpl = SanTemplate
		if initFlags.SanTmpl != "" {
			tmpl = readSourceBytes(initFlags.SanTmpl)
		}
		if err := writeFile(LOCAL, filepath.Join(sanTemplates, SanDefaultTemplate), tmpl, 0664); err != nil {
			log.Fatalln(err)
		}

		return
	}

	// cannot pass both flags
	if initFlags.Demo && initFlags.All != "" {
		return ErrInvalidArgs
	}

	if err = initGeneos(LOCAL, args); err != nil {
		log.Fatalln(err)
	}

	return
}

func initGeneos(remote RemoteName, args []string) (err error) {
	var dir string
	var uid, gid uint32
	var username, homedir string

	if remote != LOCAL && superuser {
		err = ErrNotSupported
		return
	}

	var i int
	var a string
	var params []string

	// move all args starting with first flag to params
	for i, a = range args {
		if strings.HasPrefix(a, "-") {
			params = args[i:]
			if i > 0 {
				args = args[:i]
			} else {
				args = []string{}
			}
			break
		}
	}

	// move all args to params from first on containing an '='
	if len(params) == 0 {
		for i, a = range args {
			if strings.Contains(a, "=") {
				params = args[i:]
				if i > 0 {
					args = args[:i]
				} else {
					args = []string{}
				}
				break
			}
		}
	}

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
		if remote == LOCAL {
			u, _ := user.Current()
			username = u.Username
			homedir = u.HomeDir
		} else {
			r := loadRemoteConfig(remote)
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
			dir, _ = filepath.Abs(args[0])
		default:
			logError.Fatalln("too many args:", args, params)
		}
	}

	// dir must first not exist (or be empty) and then be createable
	if _, err := statFile(remote, dir); err == nil {
		// check empty
		dirs, err := readDir(remote, dir)
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
		if err = mkdirAll(remote, dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}

	if superuser {
		if err = chown(LOCAL, dir, int(uid), int(gid)); err != nil {
			logError.Fatalln(err)
		}
	}

	// create directories
	for _, d := range initDirs {
		dir := filepath.Join(dir, d)
		if err = mkdirAll(remote, dir, 0775); err != nil {
			logError.Fatalln(err)
		}
	}

	if superuser {
		err = filepath.WalkDir(dir, func(path string, dir fs.DirEntry, err error) error {
			if err == nil {
				err = chown(LOCAL, path, int(uid), int(gid))
			}
			return err
		})
	}

	if remote == LOCAL {
		c := make(GlobalSettings)
		c["ITRSHome"] = dir
		c["DefaultUser"] = username

		if superuser {
			if err = writeConfigFile(LOCAL, globalConfig, "root", c); err != nil {
				logError.Fatalln("cannot write global config", err)
			}

			// if everything else worked, remove any existing user config
			_ = removeFile(LOCAL, filepath.Join(dir, ".config", "geneos.json"))
		} else {
			userConfDir, err := os.UserConfigDir()
			if err != nil {
				log.Fatalln(err)
			}
			userConfFile := filepath.Join(userConfDir, "geneos.json")

			if err = writeConfigFile(LOCAL, userConfFile, username, c); err != nil {
				return err
			}
		}
	}

	// now reload config, after init
	loadSysConfig()
	for _, c := range components {
		if c.Initialise != nil {
			c.Initialise(LOCAL)
		}
	}

	if initFlags.GatewayTmpl != "" {
		tmpl := readSourceBytes(initFlags.GatewayTmpl)
		if err := writeFile(LOCAL, GeneosPath(LOCAL, Gateway.String(), "templates", GatewayDefaultTemplate), tmpl, 0664); err != nil {
			log.Fatalln(err)
		}
	}

	if initFlags.SanTmpl != "" {
		tmpl := readSourceBytes(initFlags.SanTmpl)
		if err := writeFile(LOCAL, GeneosPath(LOCAL, San.String(), "templates", SanDefaultTemplate), tmpl, 0664); err != nil {
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
	r := []string{"@" + remote.String()}

	// create a demo environment
	if initFlags.Demo {
		g := []string{"Demo Gateway@" + remote.String()}
		n := []string{"localhost@" + remote.String()}
		w := []string{"demo@" + remote.String()}
		commandDownload(None, e, e)
		commandAdd(Gateway, g, e)
		commandSet(Gateway, g, []string{"GateOpts=-demo"})
		commandAdd(Netprobe, n, e)
		commandAdd(Webserver, w, e)
		// call defaultArgs() on an empty list to populate for loopCommand()
		ct, args, params := defaultArgs(r)
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
		if remote != LOCAL {
			sanname = sanname + "@" + remote.String()
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
		n := []string{"localhost@" + remote.String()}
		commandDownload(None, e, e)
		commandAdd(Licd, g, e)
		commandImport(Licd, g, []string{"geneos.lic=" + initFlags.All})
		commandAdd(Gateway, g, e)
		commandAdd(Netprobe, n, e)
		commandAdd(Webserver, g, e)
		// call defaultArgs() on an empty list to populate for loopCommand()
		ct, args, params := defaultArgs(r)
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
