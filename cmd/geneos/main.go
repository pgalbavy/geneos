package main

import (
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

func init() {
	if os.Geteuid() == 0 || os.Getuid() == 0 {
		superuser = true
	}
}

// give these more convenient names and also shadow the std log
// package for normal logging
var (
	log      = logger.Log
	DebugLog = logger.Debug
	ErrorLog = logger.Error
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
		log.Fatalln("[usage here]: not enough args")
	}

	var command = strings.ToLower(leftargs[0])
	var ct ComponentType = None
	var args []string = leftargs[1:]
	var params []string

	// parse the rest of the args depending on the command
	if commands[command].ParseArgs != nil {
		ct, args, params = commands[command].ParseArgs(leftargs[1:])
	}
	DebugLog.Println("ct", ct, "args", args, "params", params)

	// if command is not an init, set or show then the ITRSHome
	// directory must exist and be accessible to the user
	switch command {
	// come commands just want the raw command args, or none
	case "help", "version", "init":
		if err := commands[command].Function(ct, args, params); err != nil {
			log.Fatalln(err)
		}
		os.Exit(0)
	// 'geneos show [user|global]'
	case "show":
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
		if len(args) > 0 && (args[0] == "user" || args[0] == "global") {
			// output on-disk global or user config, not resolved one
			if err := commands[command].Function(ct, args, params); err != nil {
				log.Fatalln(err)
			}
			os.Exit(0)
		}
		fallthrough
	default:
		// test home dir, stop if invalid
		if RunningConfig.ITRSHome == "" {
			log.Fatalln("home directory is not set")
		}
		s, err := os.Stat(RunningConfig.ITRSHome)
		if err != nil {
			log.Fatalf("home directory %q: %s", RunningConfig.ITRSHome, errors.Unwrap(err))
		}
		if !s.IsDir() {
			log.Fatalln(RunningConfig.ITRSHome, "is not a directory")
		}

		// we have a valid home directory, now set default user if
		// not set elsewhere
		if RunningConfig.DefaultUser == "" {
			s2 := s.Sys().(*syscall.Stat_t)
			if s2.Uid == 0 {
				log.Fatalf("home directory %q: owned by root and no default user configured", RunningConfig.ITRSHome)
			}
			u, err := user.LookupId(fmt.Sprint(s2.Uid))
			if err != nil {
				log.Fatalln(RunningConfig.ITRSHome, err)
			}
			RunningConfig.DefaultUser = u.Username
		}

		//logger.EnableDebugLog()

		c, ok := commands[command]
		if !ok {
			ErrorLog.Fatalln("unknown command", command)
		}

		// the command has to understand ct == None/Unknown
		if err = c.Function(ct, args, params); err != nil {
			if err == ErrInvalidArgs {
				log.Fatalf("Usage: %q", c.CommandLine)
			}
			log.Fatalln(err)
		}
	}
	os.Exit(0)
}
