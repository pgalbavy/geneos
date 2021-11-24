package main

import (
	"log"
	"os"
	"strings"
)

func init() {
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		itrsHome = h
	}
}

//
// redo
//
// geneosctl COMMAND [COMPONENT] [NAME]
//
// COMPONENT = "" | gateway | netprobe | licd | webserver
// COMMAND = start | stop | restart | status | command | ...
//
func main() {
	if len(os.Args) < 2 {
		log.Fatalln("[usage here]: not enough args")
	}

	var command = strings.ToLower(os.Args[1])

	var comp ComponentType
	var names []string

	if len(os.Args) > 2 {
		comp = ct(os.Args[2])
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
		case "version":
		case "help":
		case "status":
			names = compAll(comp)
		default:
			os.Exit(0)
		}
	}

	// loop over names, if any supplied
	for _, name := range names {
		var c Component

		switch comp {
		case Gateway:
			c = newGateway(name)
		case Netprobe:
			c = newNetprobe(name)
		case Licd:
			c = newLicd(name)
		case Webserver:
			log.Println("webserver not supported yet")
			os.Exit(0)
		default:
			log.Println("unknown component", comp)
			os.Exit(0)
		}

		switch command {
		case "create":
			// create one or more components, type is option
			// if no type create multiple different components for each names
		case "start":
			start(c, name)
		case "stop":
			stop(c, name)
		case "restart":
			stop(c, name)
			start(c, name)
		case "command":
			cmd, env := loadConfig(c, name)
			if cmd != nil {
				log.Printf("command: %q\n", cmd.String())
				log.Println("extra environment:")
				for _, e := range env {
					log.Println(e)
				}
			}
			log.Println("end")
		case "details":
			//
		case "status":
			pid, err := findProc(c, name) // getPid(c, name)
			if err != nil {
				log.Println(Type(c), name, ":", err)
				continue
			}
			log.Println(Type(c), name, "running with PID", pid)

		case "refresh":
			// c.refresh(ct, name)
		case "log":
		case "delete":
		default:
			log.Fatalln(Type(c), "[usage here] unknown command:", command)
		}
	}
}
