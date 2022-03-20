package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"os/user"
	"strings"

	"wonderland.org/geneos/pkg/logger"
)

//go:embed VERSION
var releaseVersion string

func init() {
	if os.Geteuid() == 0 || os.Getuid() == 0 {
		superuser = true
	}
}

// give these more convenient names and also shadow the std log
// package for normal logging
var (
	log      = logger.Log
	logDebug = logger.Debug
	logError = logger.Error
)

// define standard errors for reuse
var (
	ErrNotSupported = errors.New("not supported")
	ErrPermission   = os.ErrPermission
	ErrInvalidArgs  = os.ErrInvalid
	ErrProcNotExist = errors.New("process not found")
	ErrProcExists   = errors.New("process exists")
	ErrDisabled     = errors.New("disabled")
	ErrNotFound     = fs.ErrNotExist
	ErrNoAction     = errors.New("no action taken")
)

// simple check for root
// checked to either drop privs or to set real uids etc.
var superuser bool = false

// map of all configured commands
//
// this is populated buy the init() functions in each
// command source file and the functions must
// have the same signature and be self-sufficient, returning
// only an error (or nil)
var commands Commands = make(Commands)

// command line general form(s):
//
// geneos [FLAG...] COMMAND [FLAG...] [TYPE] [NAME...] [PARAM...]
//
// Where:
//
// FLAG - parsed by the flag package
// COMMAND - in the map above
// TYPE - parsed by CompType where None means no match
// NAME - one or more instance names, matching the validNames() test
// PARAM - everything else, left after the last NAME is found
//
// Soecial case or genearlise some commands - the don't call parseArgs()
// or whatever. e.g. "geneos set global [PARAM...]"
func main() {
	var debug, quiet, version bool

	flag.BoolVar(&debug, "d", false, "enable debug output")
	flag.BoolVar(&version, "v", false, "show version")
	flag.BoolVar(&quiet, "q", false, "quiet output")
	flag.Parse()
	var leftargs = flag.Args()

	if version {
		log.Println("version:", releaseVersion)
		os.Exit(0)
	}

	if quiet {
		log.SetOutput(ioutil.Discard)
	} else if debug {
		logger.EnableDebugLog()
	}

	if len(leftargs) == 0 {
		commandHelp(None, nil, nil)
		os.Exit(0)
	}

	loadSysConfig()

	// initialise placeholder structs
	rLOCAL = NewRemote(string(LOCAL)).(*Remotes)
	rALL = NewRemote(string(ALL)).(*Remotes)

	//	log.Printf("%#v", rLOCAL)

	var command = strings.ToLower(leftargs[0])
	var ct Component = None
	var args []string = leftargs[1:]
	var params []string

	// parse the args depending on the command
	if commands[command].ParseArgs != nil {
		ct, args, params = commands[command].ParseArgs(commands[command], args)
	}
	logDebug.Println("ct", ct, "args", args, "params", params)

	// if command is not an init, set or show then the ITRSHome
	// directory must exist and be accessible to the user
	switch command {
	case "help", "version", "init":
		// some commands just want the raw command args, or none
		if err := commands[command].Function(ct, args, params); err != nil {
			logError.Fatalln(err)
		}
		os.Exit(0)
	case "show":
		// 'geneos show [user|global]'
		if ct == None {
			// check the unparsed args here
			if len(leftargs[1:]) == 0 {
				// output resolved config and exit
				printConfigStructJSON(GlobalConfig)
				os.Exit(0)
			}
		}
		// some other "show" comnbination
		fallthrough
	case "set", "edit":
		// process set or show global|user or keep going to instances
		// XXX with the new ParseArgs method, args may not be empty as it may contain the settings. need to check
		if len(args) == 0 || (len(args) > 0 && (args[0] == "user" || args[0] == "global")) {
			// output on-disk global or user config, not resolved one
			if err := commands[command].Function(ct, args, params); err != nil {
				logError.Fatalln(err)
			}
			os.Exit(0)
		}
		fallthrough
	default:
		// test home dir, stop if invalid
		if ITRSHome() == "" {
			log.Fatalln(`
Installation directory is not set.

You can fix this by doing one of the following:

1. Create a new Geneos environment:

	$ geneos init /path/to/geneos

2. Set the ITRS_HOME environment:

	$ export ITRS_HOME=/path/to/geneos

3. Set the ITRSHome user parameter:

	$ geneos set user ITRSHome=/path/to/geneos

3. Set the ITRSHome parameter in the global configuration file:

	$ echo '{ "ITRSHome": "/path/to/geneos" }' >` + globalConfig)
		}
		s, err := rLOCAL.statFile(ITRSHome())
		if err != nil {
			logError.Fatalf("home directory %q: %s", ITRSHome(), errors.Unwrap(err))
		}
		if !s.st.IsDir() {
			logError.Fatalln(ITRSHome(), "is not a directory")
		}

		// we have a valid home directory, now set default user if
		// not set elsewhere
		if GlobalConfig["DefaultUser"] == "" {
			if s.uid == 0 {
				logError.Fatalf("home directory %q: owned by root and no default user configured", ITRSHome())
			}
			u, err := user.LookupId(fmt.Sprint(s.uid))
			if err != nil {
				logError.Fatalln(ITRSHome(), err)
			}
			GlobalConfig["DefaultUser"] = u.Username
		}

		cmd, ok := commands[command]
		if !ok {
			logError.Fatalln("unknown command", command)
		}

		// the command has to understand ct == None/Unknown
		if err = cmd.Function(ct, args, params); err != nil {
			if err == ErrInvalidArgs {
				logError.Fatalf("Usage: %q", cmd.CommandLine)
			}
			logError.Fatalln(err)
		}
	}
	os.Exit(0)
}
