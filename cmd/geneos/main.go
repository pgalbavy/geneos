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
	if len(os.Args) < 2 {
		log.Fatalln("[usage here]: not enough args")
	}

	var command = strings.ToLower(os.Args[1])

	// if command is not an init, set or show then the ITRSHome
	// directory must exist and be accessible to the user
	if command != "init" && command != "set" && command != "show" {
		// test home dir, refuse to run if invalid
		s, err := os.Stat(Config.ITRSHome)
		if err != nil {
			log.Fatalln(err)
		}
		if !s.IsDir() {
			log.Fatalln(Config.ITRSHome, "is not a directory")
		}

		// we have a valid home directory, now set default user if
		// not set elsewhere
		if Config.DefaultUser == "" {
			s2 := s.Sys().(*syscall.Stat_t)
			if s2.Uid == 0 {
				log.Fatalln(Config.ITRSHome, "owned by root and no default user configured")
			}
			u, err := user.LookupId(fmt.Sprint(s2.Uid))
			if err != nil {
				log.Fatalln(Config.ITRSHome, err)
			}
			Config.DefaultUser = u.Username
		}
	}
	comp, names := parseArgs(os.Args[2:])

	//logger.EnableDebugLog()

	c, ok := commands[command]
	if !ok {
		ErrorLog.Fatalln("unknown command", command)
	}

	// the command has to understand comp == None/Unknown
	err := c.Function(comp, names)
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(0)
}
