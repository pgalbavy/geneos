package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os/user"
	"text/tabwriter"
	"time"
)

var listJSON bool
var listCSV bool
var listFlags *flag.FlagSet

func init() {
	listFlags = flag.NewFlagSet("ls", flag.ExitOnError)
	listFlags.BoolVar(&listJSON, "j", false, "Output JSON")
	listFlags.BoolVar(&listCSV, "c", false, "Output CSV")

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

var lsTabWriter *tabwriter.Writer
var csvWriter *csv.Writer
var jsonEncoder *json.Encoder

func commandLS(ct ComponentType, args []string, params []string) (err error) {
	listFlags.Parse(params)
	DebugLog.Println("JSON", listJSON)
	DebugLog.Println("CSV", listCSV)
	if listJSON && listCSV {
		log.Fatalln("only one of -j or -c allowed")
	}
	params = listFlags.Args()
	switch {
	case listJSON:
		jsonEncoder = json.NewEncoder(log.Writer())
		//jsonEncoder.SetIndent("", "    ")
		err = loopCommand(lsInstanceJSON, ct, args, params)
	case listCSV:
		csvWriter = csv.NewWriter(log.Writer())
		csvWriter.Write([]string{"Type", "Name", "Disabled", "Home"})
		err = loopCommand(lsInstanceCSV, ct, args, params)
		csvWriter.Flush()
	default:
		lsTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
		fmt.Fprintf(lsTabWriter, "Type\tName\tHome\n")
		err = loopCommand(lsInstancePlain, ct, args, params)
		lsTabWriter.Flush()
	}
	return
}

func lsInstancePlain(c Instance, params []string) (err error) {
	DebugLog.Println("params", params)
	var suffix string
	if isDisabled(c) {
		suffix = "*"
	}
	fmt.Fprintf(lsTabWriter, "%s\t%s\t%s\n", Type(c), Name(c)+suffix, Home(c))
	return
}

func lsInstanceCSV(c Instance, params []string) (err error) {
	var dis string = "N"
	if isDisabled(c) {
		dis = "Y"
	}
	csvWriter.Write([]string{Type(c).String(), Name(c), dis, Home(c)})
	return
}

type lsType struct {
	Type     string
	Name     string
	Disabled string
	Home     string
}

func lsInstanceJSON(c Instance, params []string) (err error) {
	var dis string = "N"
	if isDisabled(c) {
		dis = "Y"
	}
	jsonEncoder.Encode(lsType{Type(c).String(), Name(c), dis, Home(c)})
	return
}

// TODO: CSV and JSON versions for automation
//
// list instance processes: type, name, uid, gid, threads, starttime, directory, fds, args (?)
//

var psTabWriter *tabwriter.Writer

func commandPS(ct ComponentType, args []string, params []string) (err error) {
	listFlags.Parse(params)
	DebugLog.Println("JSON", listJSON)
	DebugLog.Println("CSV", listCSV)
	if listJSON && listCSV {
		log.Fatalln("only one of -j or -c allowed")
	}
	params = listFlags.Args()
	switch {
	case listJSON:
		jsonEncoder = json.NewEncoder(log.Writer())
		//jsonEncoder.SetIndent("", "    ")
		err = loopCommand(psInstanceJSON, ct, args, params)
	case listCSV:
		csvWriter = csv.NewWriter(log.Writer())
		csvWriter.Write([]string{"Type:Name", "PID", "User", "Group", "Starttime", "Home"})
		err = loopCommand(psInstanceCSV, ct, args, params)
		csvWriter.Flush()
	default:
		psTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
		fmt.Fprintf(psTabWriter, "Type:Name\tPID\tUser\tGroup\tStarttime\tHome\n")
		err = loopCommand(psInstancePlain, ct, args, params)
		psTabWriter.Flush()
	}
	return
}

func psInstancePlain(c Instance, params []string) (err error) {
	if isDisabled(c) {
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

	fmt.Fprintf(psTabWriter, "%s:%s\t%d\t%s\t%s\t%s\t%s\n", Type(c), Name(c), pid, username, groupname, time.Unix(st.Ctim.Sec, st.Ctim.Nsec).Local().Format(time.RFC3339), Home(c))

	return
}

func psInstanceCSV(c Instance, params []string) (err error) {
	if isDisabled(c) {
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

	csvWriter.Write([]string{Type(c).String() + ":" + Name(c), fmt.Sprint(pid), username, groupname, time.Unix(st.Ctim.Sec, st.Ctim.Nsec).Local().Format(time.RFC3339), Home(c)})

	return
}

type psType struct {
	Type      string
	Name      string
	PID       string
	User      string
	Group     string
	Starttime string
	Home      string
}

func psInstanceJSON(c Instance, params []string) (err error) {
	if isDisabled(c) {
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

	jsonEncoder.Encode(psType{Type(c).String(), Name(c), fmt.Sprint(pid), username, groupname, time.Unix(st.Ctim.Sec, st.Ctim.Nsec).Local().Format(time.RFC3339), Home(c)})

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
