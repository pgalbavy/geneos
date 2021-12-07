package main

import (
	"strconv"
	"strings"
)

func init() {
	commands["create"] = Command{commandCreate, "create"}
}

// call the component specific create functions
func commandCreate(comp ComponentType, args []string) error {
	ports := getPorts()
	log.Printf("ports=%v\n", ports)
	n := nextPort("7036..")
	log.Println("next:", n)
	return ErrNotSupported
}

// get all used ports in config files.
// this will not work for ports assigned in component config
// files, such as gateway setup or netprobe collection agent
//
// returns a map
func getPorts() (ports map[int]ComponentType) {
	ports = make(map[int]ComponentType)
	confs := allComponents() // sorting doesn't matter
	for _, comp := range confs {
		for _, c := range comp {
			if port := getIntAsString(c, Prefix(c)+"Port"); port != "" {
				p, err := strconv.Atoi(port)
				if err == nil {
					ports[int(p)] = Type(c)
				}
			}
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

// given a range, find the first unsed port
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
func nextPort(from string) int {
	used := getPorts()
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
				_, ok := used[i]
				if !ok {
					// found an unused port
					return i
				}
			}
		} else {
			p1, err := strconv.Atoi(m[0])
			if err != nil || p1 < 1 || p1 > 49151 {
				continue
			}
			_, ok := used[p1]
			if !ok {
				return p1
			}
		}
	}
	return 0
}
