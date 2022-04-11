package main

import (
	"errors"
	"flag"
	"io/fs"
	"os/user"
	"strconv"
	"strings"
)

func init() {
	RegsiterCommand(Command{
		Name:          "add",
		Function:      commandAdd,
		ParseFlags:    addFlag,
		ParseArgs:     parseArgs,
		Wildcard:      false,
		ComponentOnly: false,
		CommandLine:   "geneos add [-t FILE] TYPE NAME",
		Summary:       `Add a new instance`,
		Description: `Add a new instance called NAME with the TYPE supplied. The details will depends on the
TYPE. Currently the listening port is selected automatically and other options are defaulted. If
these need to be changed before starting, see the edit command.

Gateways are given a minimal configuration file.

FLAGS:
	-t FILE	- specify a template file to use instead of the embedded ones
	Also accepts the same flags as 'init' for remote sans
`})

	addFlags = flag.NewFlagSet("add", flag.ExitOnError)
	addFlags.StringVar(&addTemplateFile, "t", "", "template file to use instead of default")
	addFlags.BoolVar(&addStart, "S", false, "Start new instance(s) after creation")
	addFlags.BoolVar(&helpFlag, "h", false, helpUsage)

}

var addFlags *flag.FlagSet
var addTemplateFile string
var addStart bool

func addFlag(command string, args []string) []string {
	addFlags.Parse(args)
	checkHelpFlag(command)
	return addFlags.Args()
}

// Add a single instance
//
// XXX argument validation is minimal
//
// remote support would be of the form name@remotename
//
func commandAdd(ct Component, args []string, params []string) (err error) {
	var username string
	if len(args) == 0 {
		logError.Fatalln("not enough args")
	}

	// check validity and reserved words here
	name := args[0]

	_, _, rem := SplitInstanceName(name, rLOCAL)
	if err = ct.makeComponentDirs(rem); err != nil {
		return
	}

	if superuser {
		username = GlobalConfig["DefaultUser"]
	} else {
		u, _ := user.Current()
		username = u.Username
	}

	c, err := ct.GetInstance(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return
	}

	// check if instance already exists``
	if c.Loaded() {
		log.Println(c, "already exists")
		return
	}

	if err = c.Add(username, params, addTemplateFile); err != nil {
		logError.Fatalln(err)
	}

	// reload config as instance data is not updated by Add() as an interface value
	c.Unload()
	c.Load()
	log.Printf("%s added, port %d\n", c, getInt(c, c.Prefix("Port")))

	if addStart || initFlags.StartSAN {
		startInstance(c, nil)
		// commandStart(c.Type(), []string{name}, []string{})
	}

	return
}

// get all used ports in config files on a specific remote
// this will not work for ports assigned in component config
// files, such as gateway setup or netprobe collection agent
//
// returns a map
func (r *Remotes) getPorts() (ports map[int]Component) {
	if r == rALL {
		logError.Fatalln("getports() call with all remotes")
	}
	ports = make(map[int]Component)
	for _, c := range None.GetInstancesForComponent(r) {
		if !getBool(c, "ConfigLoaded") {
			log.Println("cannot load configuration for", c)
			continue
		}
		if port := getInt(c, c.Prefix("Port")); port != 0 {
			ports[int(port)] = c.Type()
		}
	}
	return
}

// syntax of ranges of ints:
// x,y,a-b,c..d m n o-p
// also open ended A,N-,B
// command or space seperated?
// - or .. = inclusive range
//
// how to represent
// split, for range, check min-max -> max > min
// repeats ignored
// special ports? - nah
//

// given a range, find the first unused port
//
// range is comma or two-dot seperated list of
// single number, e.g. "7036"
// min-max inclusive range, e.g. "7036-8036"
// start- open ended range, e.g. "7041-"
//
// some limits based on https://en.wikipedia.org/wiki/List_of_TCP_and_UDP_port_numbers
//
// not concurrency safe at this time
//
func (r *Remotes) nextPort(ct Component) int {
	from := GlobalConfig[components[ct].PortRange]
	used := r.getPorts()
	ps := strings.Split(from, ",")
	for _, p := range ps {
		// split on comma or ".."
		m := strings.SplitN(p, "-", 2)
		if len(m) == 1 {
			m = strings.SplitN(p, "..", 2)
		}

		if len(m) > 1 {
			min, err := strconv.Atoi(m[0])
			if err != nil {
				continue
			}
			if m[1] == "" {
				m[1] = "49151"
			}
			max, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			if min >= max {
				continue
			}
			for i := min; i <= max; i++ {
				if _, ok := used[i]; !ok {
					// found an unused port
					return i
				}
			}
		} else {
			p1, err := strconv.Atoi(m[0])
			if err != nil || p1 < 1 || p1 > 49151 {
				continue
			}
			if _, ok := used[p1]; !ok {
				return p1
			}
		}
	}
	return 0
}
