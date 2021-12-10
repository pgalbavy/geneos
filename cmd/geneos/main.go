package main

import (
	"errors"
	"fmt"
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

// default home. this can be moved to a global config file
// with a default to the home dir of the user running the command
// var Config.Root string = "/opt/itrs"

// map of all configured commands
//
// this is populated buy the init() functions in each
// command specific source file and the functions must
// have the same signature and be self-sufficient
var commands Commands = make(Commands)

func main() {
	loadSysConfig()

	if len(os.Args) < 2 {
		log.Fatalln("[usage here]: not enough args")
	}

	var command = strings.ToLower(os.Args[1])
	var ct ComponentType = None
	var args []string = os.Args[2:]

	// parse the rest of the args depending on the command
	if commands[command].ParseArgs != nil {
		ct, args = commands[command].ParseArgs(os.Args[2:])
	}

	// if command is not an init, set or show then the ITRSHome
	// directory must exist and be accessible to the user
	switch command {
	// come commands just want the raw command args, or none
	case "help", "version", "init":
		if err := commands[command].Function(ct, args); err != nil {
			log.Fatalln(err)
		}
		os.Exit(0)
	// 'geneos show [user|global]'
	case "show":
		if ct == None {
			// check the unparsed args here
			if len(os.Args[2:]) == 0 {
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
			if err := commands[command].Function(ct, args); err != nil {
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
		if err = c.Function(ct, args); err != nil {
			log.Fatalln(err)
		}
	}
	os.Exit(0)
}
