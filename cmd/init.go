/*
Copyright © 2022 Peter Galbavy <peter@wonderland.org>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/component"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance/gateway"
	"wonderland.org/geneos/internal/instance/licd"
	"wonderland.org/geneos/internal/instance/netprobe"
	"wonderland.org/geneos/internal/instance/san"
	"wonderland.org/geneos/internal/instance/webserver"
	"wonderland.org/geneos/internal/utils"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [-A FILE|URL|-D|-S|-T] [-n NAME] [-g FILE|URL] [-s FILE|URL] [-c CERTFILE] [-k KEYFILE] [USERNAME] [DIRECTORY] [PARAMS]",
	Short: "Initialise a Geneos installation",
	Long: `Initialise a Geneos installation by creating the directory
	hierarchy and user configuration file, with the USERNAME and
	DIRECTORY if supplied. DIRECTORY must be an absolute path and
	this is used to distinguish it from USERNAME.
	
	DIRECTORY defaults to ${HOME}/geneos for the selected user unless
	the last component of ${HOME} is 'geneos' in which case the home
	directory is used. e.g. if the user is 'geneos' and the home
	directory is '/opt/geneos' then that is used, but if it were a
	user 'itrs' which a home directory of '/home/itrs' then the
	directory 'home/itrs/geneos' would be used. This only applies
	when no DIRECTORY is explicitly supplied.
	
	When DIRECTORY is given it must be an absolute path and the
	parent directory must be writable by the user - either running
	the command or given as USERNAME.
	
	DIRECTORY, whether explicit or implied, must not exist or be
	empty of all except "dot" files and directories.
	
	When run with superuser privileges a USERNAME must be supplied
	and only the configuration file for that user is created. e.g.:
	
		sudo geneos init geneos /opt/itrs
	
	When USERNAME is supplied then the command must either be run
	with superuser privileges or be run by the same user.
	
	Any PARAMS provided are passed to the 'add' command called for
	components created.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("init called")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&initCmdAll, "all", "A", "",
		"Perform initialisation steps using provided license file and start environment")
	initCmd.Flags().BoolVarP(&initCmdMakeCerts, "makecerts", "C", false, "Create default certificates for TLS support")
	initCmd.Flags().BoolVarP(&initCmdDemo, "demo", "D", false, "Perform initialisation steps for a demo setup and start environment")
	initCmd.Flags().BoolVarP(&initCmdSAN, "san", "S", false, "Create a SAN and start")
	initCmd.Flags().BoolVarP(&initCmdTemplates, "templates", "T", false, "Overwrite/create templates from embedded (for version upgrades)")

	initCmd.Flags().StringVarP(&initCmdName, "name", "n", "", "Use the given name for instances and configurations instead of the hostname")

	initCmd.Flags().StringVarP(&initCmdImportCert, "importcert", "c", "", "signing certificate file with optional embedded private key")
	initCmd.Flags().StringVarP(&initCmdImportKey, "importkey", "k", "", "signing private key file")

	initCmd.Flags().StringVarP(&initCmdGatewayTemplate, "gatewaytemplate", "g", "", "A `gateway` template file")
	initCmd.Flags().StringVarP(&initCmdSANTemplate, "santemplate", "s", "", "A `san` template file")
}

var initCmdAll string
var initCmdMakeCerts, initCmdDemo, initCmdSAN, initCmdTemplates bool
var initCmdName, initCmdImportCert, initCmdImportKey, initCmdGatewayTemplate, initCmdSANTemplate string

//
// initialise a geneos installation
//
// if no directory given and not running as root and the last component of the user's
// home directory is NOT "geneos" then create a directory "geneos", else
//
// XXX Call any registered initialiser funcs from components
//
func commandInit(ct component.ComponentType, args []string, params []string) (err error) {
	// none of the arguments can be a reserved type
	if ct != component.None {
		return ErrInvalidArgs
	}

	f := initCmd.Flags()

	// rewrite local templates and exit
	if t, _ := f.GetBool("templates"); t {
		gatewayTemplates := host.LOCAL.GeneosPath(gateway.Gateway.String(), "templates")
		host.LOCAL.MkdirAll(gatewayTemplates, 0775)
		tmpl := gateway.GatewayTemplate
		if initCmdGatewayTemplate != "" {
			if tmpl, err = component.ReadLocalFileOrURL(initCmdGatewayTemplate); err != nil {
				return
			}
		}
		if err := host.LOCAL.WriteFile(filepath.Join(gatewayTemplates, gateway.GatewayDefaultTemplate), tmpl, 0664); err != nil {
			logError.Fatalln(err)
		}

		tmpl = gateway.InstanceTemplate
		if err := host.LOCAL.WriteFile(filepath.Join(gatewayTemplates, gateway.GatewayInstanceTemplate), tmpl, 0664); err != nil {
			logError.Fatalln(err)
		}

		sanTemplates := host.LOCAL.GeneosPath(san.San.String(), "templates")
		host.LOCAL.MkdirAll(sanTemplates, 0775)
		tmpl = san.SanTemplate
		if initCmdSANTemplate != "" {
			if tmpl, err = component.ReadLocalFileOrURL(initCmdSANTemplate); err != nil {
				return
			}
		}
		if err := host.LOCAL.WriteFile(filepath.Join(sanTemplates, san.SanDefaultTemplate), tmpl, 0664); err != nil {
			logError.Fatalln(err)
		}

		return
	}

	flagcount := 0
	for _, b := range []bool{initCmdDemo, initCmdTemplates, initCmdSAN} {
		if b {
			flagcount++
		}
	}

	if initCmdAll != "" {
		flagcount++
	}

	if flagcount > 1 {
		return fmt.Errorf("%w: Only one of -A, -D, -S or -T can be given", ErrInvalidArgs)
	}

	if err = initGeneos(host.LOCAL, args); err != nil {
		logError.Fatalln(err)
	}

	return
}

func initGeneos(r *host.Host, args []string) (err error) {
	var dir string
	var uid, gid int
	var username, homedir string
	var params []string

	if r != host.LOCAL && utils.IsSuperuser() {
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

	if utils.IsSuperuser() {
		if len(args) == 0 {
			logError.Fatalln("init requires a USERNAME when run as root")
		}
		username = args[0]
		uid, gid, _, err = utils.GetUser(username)

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
		if r == host.LOCAL {
			u, _ := user.Current()
			username = u.Username
			homedir = u.HomeDir
		} else {
			username = r.Username()
			homedir = r.GeneosHome()
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
				if r != host.LOCAL {
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

	if r == host.LOCAL {
		c := make(GlobalSettings)
		c["Geneos"] = dir
		c["DefaultUser"] = username

		if utils.IsSuperuser() {
			if err = host.LOCAL.writeConfigFile(globalConfig, "root", 0664, c); err != nil {
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

			if err = host.LOCAL.writeConfigFile(userConfFile, username, 0664, c); err != nil {
				return err
			}
		}
	}

	// now reload config, after init
	loadSysConfig()

	// also recreate host.LOCAL to load Geneos and others
	host.LOCAL.Unload()
	host.LOCAL = host.New(host.LOCALHOST).(*host.Host)

	if utils.IsSuperuser() {
		if err = host.LOCAL.Chown(dir, uid, gid); err != nil {
			logError.Fatalln(err)
		}
	}

	if err = component.MakeComponentDirs(host.LOCAL, component.None); err != nil {
		return
	}

	if utils.IsSuperuser() {
		err = filepath.WalkDir(dir, func(path string, dir fs.DirEntry, err error) error {
			if err == nil {
				err = host.LOCAL.Chown(path, uid, gid)
			}
			return err
		})
	}

	for _, c := range components {
		if c.Initialise != nil {
			c.Initialise(host.LOCAL)
		}
	}

	if initCmdGatewayTemplate != "" {
		var tmpl []byte
		if tmpl, err = component.ReadLocalFileOrURL(initCmdGatewayTemplate); err != nil {
			return
		}
		if err := host.LOCAL.WriteFile(host.LOCAL.GeneosPath(gateway.Gateway.String(), "templates", gateway.GatewayDefaultTemplate), tmpl, 0664); err != nil {
			logError.Fatalln(err)
		}
	}

	if initCmdSANTemplate != "" {
		var tmpl []byte
		if tmpl, err = component.ReadLocalFileOrURL(initCmdSANTemplate); err != nil {
			return
		}
		if err = host.LOCAL.WriteFile(host.LOCAL.GeneosPath(san.San.String(), "templates", san.SanDefaultTemplate), tmpl, 0664); err != nil {
			return
		}
	}

	if initCmdMakeCerts {
		TLSInit()
	} else {
		// both options can import arbitrary PEM files, fix this
		if initCmdImportCert != "" {
			TLSImport(initCmdImportCert)
		}

		if initCmdImportKey != "" {
			TLSImport(initCmdImportKey)
		}
	}
	e := []string{}
	rem := []string{"@" + r.String()}

	// create a demo environment
	if initCmdDemo {
		g := []string{"Demo Gateway@" + r.String()}
		n := []string{"localhost@" + r.String()}
		w := []string{"demo@" + r.String()}
		commandInstall(gateway.Gateway, e, e)
		commandAdd(gateway.Gateway, g, params)
		commandSet(gateway.Gateway, g, []string{"GateOpts=-demo"})
		commandInstall(san.San, e, e)
		commandAdd(san.San, n, []string{"Gateways=localhost"})
		commandInstall(webserver.Webserver, e, e)
		commandAdd(webserver.Webserver, w, params)
		// call parseArgs() on an empty list to populate for loopCommand()
		ct, args, params := parseArgs(commands["start"], rem)
		commandStart(ct, args, params)
		commandPS(ct, args, params)
		return
	}

	if initCmdSAN {
		var sanname string
		var s []string

		if initCmdName != "" {
			sanname = initCmdName
		} else {
			sanname, _ = os.Hostname()
		}
		if r != host.LOCAL {
			sanname = sanname + "@" + r.String()
		}
		s = []string{sanname}
		// Add will also install the right package
		commandAdd(san.San, s, params)
		return nil
	}

	// create a basic environment with license file
	if initCmdAll != "" {
		if initCmdName == "" {
			initCmdName, err = os.Hostname()
			if err != nil {
				return err
			}
		}
		name := []string{initCmdName}
		localhost := []string{"localhost@" + r.String()}
		commandInstall(licd.Licd, e, e)
		commandAdd(licd.Licd, name, params)
		commandImport(licd.Licd, name, []string{"geneos.lic=" + initCmdAll})
		commandInstall(gateway.Gateway, e, e)
		commandAdd(gateway.Gateway, name, params)
		commandInstall(netprobe.Netprobe, e, e)
		commandAdd(netprobe.Netprobe, localhost, params)
		commandInstall(webserver.Webserver, e, e)
		commandAdd(webserver.Webserver, name, params)
		// call parseArgs() on an empty list to populate for loopCommand()
		ct, args, params := parseArgs(commands["start"], rem)
		commandStart(ct, args, params)
		commandPS(ct, args, params)
		return nil
	}

	return
}
