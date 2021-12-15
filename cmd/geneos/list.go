package main

import (
	"fmt"
	"os/user"
	"time"
)

func init() {

	commands["ls"] = Command{commandLS, parseArgs, "geneos ls [TYPE] [NAME...]",
		`List the matching instances and their component type.

Future versions will support CSV or JSON output formats for automation and monitoring.`}

	commands["list"] = Command{commandLS, parseArgs, "geneos list [TYPE] [NAME...]", `See 'geneos ls' command`}

	commands["ps"] = Command{commandPS, parseArgs, "geneos ps [TYPE] [NAMES...]",
		`Show the status of the matching instances.

Future versions will support CSV or JSON output formats for automation and monitoring.`}
	commands["status"] = Command{commandPS, parseArgs, "geneos status [TYPE] [NAMES...]", `See 'geneos ps' command`}

	commands["command"] = Command{commandCommand, parseArgs, "geneos command [TYPE] [NAME...]",
		`Show the full command line for the matching instances along with any environment variables
explicitly set for execution.

Future versions will support CSV or JSON output formats for automation and monitoring.`}

}

func commandLS(ct ComponentType, args []string, params []string) error {
	return loopCommand(lsInstance, ct, args, params)
}

func lsInstance(c Instance, params []string) (err error) {
	log.Println(Type(c), Name(c), Home(c))
	return
}

// TODO: CSV and JSON versions for automation
//
// list instance processes: type, name, uid, gid, threads, starttime, directory, fds, args (?)
//
func commandPS(ct ComponentType, args []string, params []string) error {
	log.Println("Instance PID User Group Starttime Directory")
	return loopCommand(psInstance, ct, args, params)
}

func psInstance(c Instance, params []string) (err error) {
	if isDisabled(c) {
		// log.Println(Type(c), Name(c), ErrDisabled)
		return nil
	}
	pid, st, err := findInstanceProc(c)
	if err != nil {
		return nil
	}

	var u *user.User
	var g *user.Group

	username := fmt.Sprint(st.Uid)
	groupname := fmt.Sprint(st.Gid)

	if u, err = user.LookupId(fmt.Sprint(st.Uid)); err == nil {
		username = u.Username
	}
	if g, err = user.LookupGroupId(fmt.Sprint(st.Gid)); err == nil {
		groupname = g.Name
	}

	log.Printf("%s:%s %d %s %s %s %s\n", Type(c), Name(c), pid, username, groupname, time.Unix(st.Ctim.Sec, st.Ctim.Nsec).Local().Format(time.RFC3339), Home(c))

	return
}

func commandCommand(ct ComponentType, args []string, params []string) (err error) {
	for _, name := range args {
		for _, c := range NewComponent(ct, name) {
			if err = loadConfig(c, false); err != nil {
				log.Println("cannot load configuration for", Type(c), Name(c))
				return
			}
			commandInstance(c, params)
		}
	}
	return
}

func commandInstance(c Instance, params []string) {
	cmd, env := buildCommand(c)
	if cmd != nil {
		log.Printf("command: %q\n", cmd.String())
		log.Println("env:")
		for _, e := range env {
			log.Println(e)
		}
	}
}
