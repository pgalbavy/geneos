package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/pkg/logger"
)

// given a list of args (after command has been seen), check if first
// arg is a component type and depdup the names. A name of "all" will
// will override the rest and result in a lookup being done
//
// args with an '=' should be checked and only allowed if there are names?
//
// support glob style wildcards for instance names - allow through, let loopCommand*
// deal with them
//
// process command args in a standard way
// flags will have been handled by another function before this one
// any args with '=' are treated as parameters
//
// a bare argument with a '@' prefix means all instance of type on a remote
//
func parseArgs(cmd *cobra.Command, rawargs []string) (ct *geneos.Component, args []string, params []string) {
	var wild bool
	var newnames []string

	a := cmd.Annotations

	if len(rawargs) == 0 && a["Wildcard"] != "true" {
		return
	}

	logger.Debug.Println("rawargs, params", rawargs, params)

	// filter in place - pull out all args containing '=' into params
	n := 0
	for _, a := range rawargs {
		if strings.Contains(a, "=") {
			params = append(params, a)
		} else {
			rawargs[n] = a
			n++
		}
	}
	rawargs = rawargs[:n]

	logger.Debug.Println("rawargs, params", rawargs, params)

	a["ct"] = "none"

	if a["Wildcard"] != "true" {
		if len(rawargs) == 0 {
			ct = nil
			return
		}
		if ct = geneos.ParseComponentName(rawargs[0]); ct == nil {
			args = rawargs
			return
		}
		args = rawargs[1:]
	} else {
		// work through wildcard options
		if len(rawargs) == 0 {
			// no more arguments? wildcard everything
			ct = nil
		} else if ct = geneos.ParseComponentName(rawargs[0]); ct == nil {
			// first arg is not a known type, so treat the rest as instance names
			ct = nil
			args = rawargs
		} else {
			a["ct"] = rawargs[0]
			args = rawargs[1:]
		}

		if a["ComponentOnly"] == "true" {
			return
		}

		if len(args) == 0 {
			// no args means all instances
			wild = true
			args = instance.FindNames(host.ALL, ct)
		} else {
			// expand each arg and save results to a new slice
			// if local == "", then all instances on remote (e.g. @remote)
			// if remote == "all" (or none given), then check instance on all remotes
			// @all is not valid - should be no arg
			var nargs []string
			for _, arg := range args {
				// check if not valid first and leave unchanged, skip
				if !(strings.HasPrefix(arg, "@") || instance.ValidInstanceName(arg)) {
					logger.Debug.Println("leaving unchanged:", arg)
					nargs = append(nargs, arg)
					continue
				}
				_, local, r := instance.SplitName(arg, host.ALL)
				if !r.Loaded() {
					logger.Debug.Println(arg, "- remote not found")
					// we have tried to match something and it may result in an empty list
					// so don't re-process
					wild = true
					continue
				}

				logger.Debug.Println("split", arg, "into:", local, r.String())
				if local == "" {
					// only a '@remote' in arg
					if r.Loaded() {
						rargs := instance.FindNames(r, ct)
						nargs = append(nargs, rargs...)
						wild = true
					}
				} else if r == host.ALL {
					// no '@remote' in arg
					var matched bool
					for _, rem := range host.AllHosts() {
						wild = true
						logger.Debug.Printf("checking remote %s for %s", rem.String(), local)
						name := local + "@" + rem.String()
						if ct == nil {
							for _, cr := range geneos.RealComponents() {
								if i, err := instance.GetInstance(cr, name); err == nil && i.Loaded() {
									nargs = append(nargs, name)
									matched = true
								}
							}
						} else if i, err := instance.GetInstance(ct, name); err == nil && i.Loaded() {
							nargs = append(nargs, name)
							matched = true
						}
					}
					if !matched && instance.ValidInstanceName(arg) {
						// move the unknown unchanged - file or url - arg so it can later be pushed to params
						// do not set 'wild' though?
						logger.Debug.Println(arg, "not found, saving to params")
						nargs = append(nargs, arg)
					}
				} else {
					// save unchanged arg, may be param
					nargs = append(nargs, arg)
					// wild = true
				}
			}
			args = nargs
		}
	}

	logger.Debug.Println("ct, args, params", ct, args, params)

	m := make(map[string]bool, len(args))
	// traditional loop because we can't modify args in a loop to skip
	for i := 0; i < len(args); i++ {
		name := args[i]
		// filter name here
		if !wild && instance.ReservedName(name) {
			logError.Fatalf("%q is reserved name", name)
		}
		// move unknown args to params
		if !instance.ValidInstanceName(name) {
			params = append(params, name)
			continue
		}
		// ignore duplicates (not params above)
		if m[name] {
			continue
		}
		newnames = append(newnames, name)
		m[name] = true
	}
	args = newnames

	a["args"] = strings.Join(args, ",")
	a["params"] = strings.Join(params, ",")

	if a["Wildcard"] != "true" {
		return
	}

	// if args is empty, find them all again. ct == None too?
	if len(args) == 0 && Geneos() != "" && !wild {
		args = instance.FindNames(host.ALL, ct)
		a["args"] = strings.Join(args, ",")
	}

	logger.Debug.Println("ct, args, params", ct, args, params)
	return
}
