package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func init() {
	commands["set"] = Command{
		Function:   commandSet,
		ParseFlags: defaultFlag,
		ParseArgs:  defaultArgs,
		CommandLine: `geneos set [global|user] KEY=VALUE [KEY=VALUE...]
	geneos set [TYPE] [NAME...] KEY=VALUE [KEY=VALUE...]`,
		Summary: `Set runtime, global, user or instance configuration parameters`,
		Description: `Set configuration item values in global, user, or for a specific
instance.

To set enironment variables for an instance use the key Env and the
value var=value. Each new var=value is additive or overwrites an existing
entry for 'var', e.g.

	geneos set netprobe localhost Env=JAVA_HOME=/usr/lib/jre
	geneos set netprobe localhost Env=ORACLE_HOME=/opt/oracle

To remove an environment variable prefix the name with a hyphen '-', e.g.

	geneos set netprobe localhost Env=-JAVA_HOME

To add an include file to an auto-generated gateway use a similar syntax to the above, but in the form:

	geneos set gateway gateway1 Includes=100:path/to/include.xml
	geneos set gateway gateway1 Includes=-100

Then rebuild the configuration as required.

`}
}

// components - parse the args again and load/print the config,
// but allow for RC files again
//
// consume component names, stop at first parameter, error out if more names
func commandSet(ct Component, args []string, params []string) (err error) {
	var instances []Instances

	logDebug.Println("args", args, "params", params)

	if len(args) == 0 && len(params) == 0 {
		return ErrInvalidArgs
	}

	if ct != None && len(args) == 0 {
		// if all args have no become params (e.g. 'set gateway X=Y') then reprocess args here
		args = ct.instanceNames(ALL)
	} else if len(args) == 0 || args[0] == "user" {
		userConfDir, _ := os.UserConfigDir()
		return writeConfigParams(filepath.Join(userConfDir, "geneos.json"), params)
	} else if args[0] == "global" {
		return writeConfigParams(globalConfig, params)
	}

	// loop through named instances
	for _, arg := range args {
		instances = append(instances, ct.instanceMatches(arg)...)
	}

	for _, arg := range params {
		// special handling for "Env" field, which is always
		// a slice of environment key=value pairs
		// 'geneos set probe Env=JAVA_HOME=/path'
		// remove with leading '-' ?
		// 'geneos set probe Env=-PASSWORD'
		s := strings.SplitN(arg, "=", 2)
		if len(s) != 2 {
			logError.Printf("ignoring %q %s", arg, ErrInvalidArgs)
			continue
		}
		k, v := s[0], s[1]

		defaults := map[string]string{
			"Includes": "100",
			"Gateways": "7039",
		}

		// loop through all provided instances, set the parameter(s)
		for _, c := range instances {
			for _, vs := range strings.Split(v, ",") {
				switch k {
				// make this list dynamic
				case "Includes", "Gateways":
					var remove bool
					e := strings.SplitN(vs, ":", 2)
					if strings.HasPrefix(e[0], "-") {
						e[0] = strings.TrimPrefix(e[0], "-")
						remove = true
					}
					if remove {
						err = setStructMap(c, k, e[0], "")
						if err != nil {
							logDebug.Printf("%s %s@%s: delete %v[%v] failed, %s", c.Type(), c.Name(), c.Location(), k, e[0], err)
						}
					} else {
						val := defaults[k]
						if len(e) > 1 {
							val = e[1]
						}
						err = setStructMap(c, k, e[0], val)
						if err != nil {
							logDebug.Printf("%s %s@%s: set %v[%v]=%v failed, %s", c.Type(), c.Name(), c.Location(), k, e[0], val, err)
						}
					}
				case "Attributes":
					var remove bool
					e := strings.SplitN(vs, "=", 2)
					if strings.HasPrefix(e[0], "-") {
						e[0] = strings.TrimPrefix(e[0], "-")
						remove = true
					}
					// '-name' or 'name=' remove the attribute
					if remove || len(e) == 1 {
						err = setStructMap(c, k, e[0], "")
						if err != nil {
							logDebug.Printf("%s %s@%s: delete %v[%v] failed, %s", c.Type(), c.Name(), c.Location(), k, e[0], err)
						}
					} else {
						err = setStructMap(c, k, e[0], e[1])
						if err != nil {
							logDebug.Printf("%s %s@%s: set %v[%v]=%v failed, %s", c.Type(), c.Name(), c.Location(), k, e[0], e[1], err)
						}
					}
				case "Env", "Types":
					var remove bool
					slice := getSliceStrings(c, k)
					e := strings.SplitN(vs, "=", 2)
					if strings.HasPrefix(e[0], "-") {
						e[0] = strings.TrimPrefix(e[0], "-")
						remove = true
					}
					anchor := "="
					if remove && strings.HasSuffix(e[0], "*") {
						// wildcard removal (only)
						e[0] = strings.TrimSuffix(e[0], "*")
						anchor = ""
					}
					var exists bool
					// transfer ietms to new slice as removing items in a loop
					// does random things
					var newslice []string
					for _, n := range slice {
						if strings.HasPrefix(n, e[0]+anchor) {
							if !remove {
								// replace with new value
								newslice = append(newslice, vs)
								exists = true
							}
						} else {
							// copy existing
							newslice = append(newslice, n)
						}
					}
					// add a new item rather than update or remove
					if !exists && !remove {
						newslice = append(newslice, vs)
					}
					if err = setFieldSlice(c, k, newslice); err != nil {
						logDebug.Printf("%s %s@%s: set %s=%s failed, %s", c.Type(), c.Name(), c.Location(), k, newslice, err)
					}
				default:
					if err = setField(c, k, vs); err != nil {
						logDebug.Printf("%s %s@%s: set %s=%s failed, %s", c.Type(), c.Name(), c.Location(), k, vs, err)
					}
				}
			}
		}
	}

	// now loop through the collected results and write out
	for _, c := range instances {
		if err = migrateConfig(c); err != nil {
			log.Fatalln("cannot migrate existing .rc config to set values in new .json configration file:", err)
		}
		if err = writeInstanceConfig(c); err != nil {
			log.Fatalln(err)
		}
	}

	return
}

func writeConfigParams(filename string, params []string) (err error) {
	var c GlobalSettings
	// ignore err - config may not exist, but that's OK
	_ = readConfigFile(LOCAL, filename, &c)
	// change here
	if len(c) == 0 {
		c = make(GlobalSettings)
	}
	for _, set := range params {
		// skip all non '=' args
		if !strings.Contains(set, "=") {
			continue
		}
		s := strings.SplitN(set, "=", 2)
		k, v := s[0], s[1]
		c[Global(k)] = v
	}

	// XXX fix permissions assumptions here
	if filename == globalConfig {
		return writeConfigFile(LOCAL, filename, "root", c)
	}
	return writeConfigFile(LOCAL, filename, "", c)
}

// check for rc file? migrate?
func writeInstanceConfig(c Instances) (err error) {
	err = writeConfigFile(c.Location(), filepath.Join(c.Home(), c.Type().String()+".json"), c.Prefix("User"), c)
	return
}

// try to be atomic, lots of edge cases, UNIX/Linux only
// we know the size of config structs is typicall small, so just marshal
// in memory
func writeConfigFile(remote RemoteName, file string, username string, config interface{}) (err error) {
	j, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return
	}

	uid, gid := -1, -1
	if superuser {
		if username == "" {
			// try $SUDO_UID etc.
			sudoUID := os.Getenv("SUDO_UID")
			sudoGID := os.Getenv("SUDO_GID")

			if sudoUID != "" && sudoGID != "" {
				if uid, err = strconv.Atoi(sudoUID); err != nil {
					uid = -1
				}

				if gid, err = strconv.Atoi(sudoGID); err != nil {
					gid = -1
				}
			}
		} else {
			uid, gid, _, _ = getUser(username)
		}
	}

	dir := filepath.Dir(file)
	// try to ensure directory exists
	if err = mkdirAll(remote, dir, 0775); err != nil {
		return
	}
	// change final directory ownership
	_ = chown(remote, dir, uid, gid)

	buffer := bytes.NewBuffer(j)
	f, fn, err := createTempFile(remote, file, 0664)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = chown(remote, fn, uid, gid); err != nil {
		removeFile(remote, fn)
	}

	if _, err = io.Copy(f, buffer); err != nil {
		return err
	}

	return renameFile(remote, fn, file)

}
