package main

import (
	"os"
	"strings"

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

// simple check for root
var superuser bool = false

// default home. this can be moved to a global config file
// with a default to the home dir of the user running the command
// var Config.Root string = "/opt/itrs"

// map of all configured commands
//
// this is populated buy the init() functions in each
// command specific source file and the functions must
// have the same signature and be self-sufficient
var commands Command = make(Command)

func main() {
	if len(os.Args) < 2 {
		log.Fatalln("[usage here]: not enough args")
	}

	var command = strings.ToLower(os.Args[1])

	comp, names := parseArgs(os.Args[2:])

	//logger.EnableDebugLog()

	c, ok := commands[command]
	if !ok {
		ErrorLog.Fatalln("unknown command", command)
	}
	// the command has to understand comp == None/Unknown
	c(comp, names)
	os.Exit(0)
}
