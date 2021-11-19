package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"syscall"
)

type Components interface {
	all() []string
	list()

	setup(name string) (cmd *exec.Cmd, env []string)

	start(name string)
	stop(name string)

	getPid(name string) (pid int, pidFile string, err error)
}

type ComponentType int

const (
	Any ComponentType = iota
	Gateway
	Netprobe
	Licd
	Webserver
)

func (ct ComponentType) String() string {
	switch ct {
	case Any:
		return "any"
	case Gateway:
		return "gateway"
	case Netprobe:
		return "netprobe"
	case Licd:
		return "licd"
	case Webserver:
		return "webserver"
	}
	return "unknown"
}

func init() {
	if h, ok := os.LookupEnv("ITRS_HOME"); ok {
		itrsHome = h
	}
}

func main() {
	var ct ComponentType
	var c Components

	if len(os.Args) < 3 {
		log.Fatalln("not enough args")
	}

	switch os.Args[1] {
	case "all", "any":
		ct = Any
		//
	case "gateway":
		ct = Gateway
		c = NewGateway()
	case "netprobe", "probe":
		ct = Netprobe
		c = NewNetprobe()
	case "licd":
		ct = Licd
		c = NewLicd()
	case "webserver", "webdashboard", "dashboard":
		ct = Webserver
		log.Println("webserver not supported yet")
		os.Exit(0)
	case "list":
		// list all compoents, annotated
	default:
		log.Println("unknown component", os.Args[1])
		os.Exit(0)
	}

	switch os.Args[2] {
	case "list":
		c.list()
		os.Exit(0)
	case "create":
		// createGateway()
		os.Exit(0)
	}

	if len(os.Args) < 4 {
		log.Fatalln("not enough args and lot list or create")
	}

	var action = os.Args[3]

	names := []string{os.Args[2]}
	if os.Args[2] == "all" {
		names = c.all()
	}

	for _, name := range names {
		switch action {
		case "start":
			c.start(name)
		case "stop":
			c.stop(name)
		case "restart":
			c.stop(name)
			c.start(name)
		case "command":
			cmd, env := c.setup(name)
			if cmd != nil {
				log.Printf("command: %q\n", cmd.String())
				log.Println("extra environment:")
				for _, e := range env {
					log.Println(e)
				}
			}
		case "details":
			//
		case "status":
			pid, _, err := c.getPid(name)
			if err != nil {
				log.Println(ct, name, "- no valid PID file found")
				continue
			}
			proc, _ := os.FindProcess(pid)
			err = proc.Signal(syscall.Signal(0))
			//
			if err != nil && !errors.Is(err, syscall.EPERM) {
				log.Println(ct, name, "process not found", pid)
			} else {
				log.Println(ct, name, "running with PID", pid)
			}
		case "refresh":
			refresh(c, ct, name)
		case "log":

		case "delete":

		default:
			log.Fatalln(ct, "unknown action:", action)
		}
	}
}

func dirs(dir string) []string {
	files, _ := os.ReadDir(dir)
	components := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			components = append(components, file.Name())
		}
	}
	return components
}
