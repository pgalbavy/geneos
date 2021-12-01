package main

import (
	"os"
	"strings"

	"wonderland.org/geneos/pkg/logger"
)

func init() {
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		itrsHome = h
	}
	// non-Windows for now
	if os.Geteuid() == 0 || os.Getuid() == 0 {
		superuser = true
	}
}

var (
	log      = logger.Log
	DebugLog = logger.LogDebug
	ErrorLog = logger.LogError
)

var superuser bool = false

// default home. this can be moved to a global config file
// with a default to the home dir of the user running the command
var itrsHome string = "/opt/itrs"

// map of all configured commands
//
// this is populated buy the init() functions in each
// command specific source file and the functions must
// have the same signature and be self-sufficient
var commands Command = make(Command)

//
// redo
//
// geneosctl COMMAND [COMPONENT] [NAME]
//
// COMPONENT = "" | gateway | netprobe | licd | webserver
// COMMAND = start | stop | restart | status | command | ...
//   create | activate | install | update | list
//
// There are commands with and without side-effects, offer a flag to control
// side-effect calls
//

func main() {
	if len(os.Args) < 2 {
		log.Fatalln("[usage here]: not enough args")
	}

	var command = strings.ToLower(os.Args[1])

	comp, names := parseArgs(os.Args[2:])

	c, ok := commands[command]
	if !ok {
		ErrorLog.Fatalln("unknown command", command)
	}
	c(comp, names)
	os.Exit(0)
}
