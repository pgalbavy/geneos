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
}

var (
	log         = logger.Logger
	DebugLogger = logger.DebugLogger
	ErrorLogger = logger.ErrorLogger
)

//
// redo
//
// geneosctl COMMAND [COMPONENT] [NAME]
//
// COMPONENT = "" | gateway | netprobe | licd | webserver
// COMMAND = start | stop | restart | status | command | ...
//   create | activate | install | update | list
//
func main() {
	if len(os.Args) < 2 {
		log.Fatalln("[usage here]: not enough args")
	}

	var command = strings.ToLower(os.Args[1])

	var comp ComponentType
	var names []string

	if len(os.Args) > 2 {
		comp = CompType(os.Args[2])
		if comp == None {
			// this may be a name instead
			names = os.Args[2:]
		} else {
			names = os.Args[3:]
		}
	}

	if len(names) == 0 {
		// no names, check special commands and exit
		switch command {
		case "list":
			confs := getAllConfigs()
			for _, c := range confs {
				log.Printf("%s => %q\n", Type(c), Name(c))
			}
		case "version":
		case "help":
		case "status":
			confs := getAllConfigs()
			for _, c := range confs {
				pid, err := findProc(c)
				if err != nil {
					log.Println(Type(c), Name(c), err)
					continue
				}
				log.Println(Type(c), Name(c), "PID", pid)
			}
		default:
			os.Exit(0)
		}
	}

	if len(names) > 1 {
		// make sure names are unique
		m := make(map[string]bool, len(names))
		for _, name := range names {
			m[name] = true
		}
		names = nil
		for name := range m {
			names = append(names, name)

		}
	}

	// loop over names, if any supplied
	for _, name := range names {
		c := New(comp, name)

		switch command {
		case "create":
			err := create(c)
			if err != nil {
				log.Println("cannot create", comp, name, ":", err)
			}
		case "start":
			start(c)
		case "stop":
			stop(c)
		case "restart":
			stop(c)
			start(c)
		case "command":
			cmd, env := makeCmd(c)
			if cmd != nil {
				log.Printf("command: %q\n", cmd.String())
				log.Println("environment:")
				for _, e := range env {
					log.Println(e)
				}
			}
			log.Println("end")
		case "details":
			//
		case "status":
			pid, err := findProc(c) // getPid(c, name)
			if err != nil {
				log.Println(Type(c), Name(c), ":", err)
				continue
			}
			log.Println(Type(c), Name(c), "running with PID", pid)

		case "refresh":
			refresh(c)
		case "log":
		case "delete":
		default:
			log.Fatalln(Type(c), "[usage here] unknown command:", command)
		}
	}
}
