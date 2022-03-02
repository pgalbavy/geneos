package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strings"
	"syscall"

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
	var debug, quiet, verbose bool

	flag.BoolVar(&debug, "d", false, "enable debug output")
	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.BoolVar(&quiet, "q", false, "quiet output")
	flag.Parse()
	var leftargs = flag.Args()

	if quiet {
		log.SetOutput(ioutil.Discard)
	} else if debug {
		logger.EnableDebugLog()
	} else if verbose {
		log.Println("look at ME!")
	}

	loadSysConfig()

	if len(leftargs) == 0 {
		logError.Fatalln("[usage here]: not enough args")
	}

	var command = strings.ToLower(leftargs[0])
	var ct ComponentType = None
	var args []string = leftargs[1:]
	var params []string

	if commands[command].ParseFlags != nil {
		args = commands[command].ParseFlags(command, args)
	}

	// parse the rest of the args depending on the command
	if commands[command].ParseArgs != nil {
		ct, args, params = commands[command].ParseArgs(args)
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
				printConfigStructJSON(RunningConfig)
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
		if RunningConfig.ITRSHome == "" {
			logError.Fatalln("home directory is not set")
		}
		s, err := statFile(LOCAL, RunningConfig.ITRSHome)
		if err != nil {
			logError.Fatalf("home directory %q: %s", RunningConfig.ITRSHome, errors.Unwrap(err))
		}
		if !s.IsDir() {
			logError.Fatalln(RunningConfig.ITRSHome, "is not a directory")
		}

		// we have a valid home directory, now set default user if
		// not set elsewhere
		if RunningConfig.DefaultUser == "" {
			s2 := s.Sys().(*syscall.Stat_t)
			if s2.Uid == 0 {
				logError.Fatalf("home directory %q: owned by root and no default user configured", RunningConfig.ITRSHome)
			}
			u, err := user.LookupId(fmt.Sprint(s2.Uid))
			if err != nil {
				logError.Fatalln(RunningConfig.ITRSHome, err)
			}
			RunningConfig.DefaultUser = u.Username
		}

		//logger.EnableDebugLog()

		c, ok := commands[command]
		if !ok {
			logError.Fatalln("unknown command", command)
		}

		// the command has to understand ct == None/Unknown
		if err = c.Function(ct, args, params); err != nil {
			if err == ErrInvalidArgs {
				logError.Fatalf("Usage: %q", c.CommandLine)
			}
			logError.Fatalln(err)
		}
	}
	os.Exit(0)
}
