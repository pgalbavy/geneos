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

func init() {
	commands["ls"] = Command{
		Function:    commandLS,
		ParseFlags:  flagsList,
		ParseArgs:   parseArgs,
		CommandLine: "geneos ls [-c|-j] [TYPE] [NAME...]",
		Summary:     `List instances, optionally in CSV or JSON format.`,
		Description: `List the matching instances and their component type.
	
Future versions will support CSV or JSON output formats for automation and monitoring.`}

	commands["ps"] = Command{
		Function:    commandPS,
		ParseFlags:  flagsList,
		ParseArgs:   parseArgs,
		CommandLine: "geneos ps [-c|-j] [TYPE] [NAMES...]",
		Summary:     `List process information for instances, optionally in CSV or JSON format.`,
		Description: `Show the status of the matching instances.

Future versions will support CSV or JSON output formats for automation and monitoring.`}

	commands["command"] = Command{
		Function:    commandCommand,
		ParseFlags:  nil,
		ParseArgs:   parseArgs,
		CommandLine: "geneos command [TYPE] [NAME...]",
		Summary:     `Show command arguments and environment for instances.`,
		Description: `Show the full command line for the matching instances along with any environment variables
explicitly set for execution.

Future versions will support CSV or JSON output formats for automation and monitoring.`}

	listFlags = flag.NewFlagSet("ls", flag.ExitOnError)
	listFlags.BoolVar(&listJSON, "j", false, "Output JSON")
	listFlags.BoolVar(&listJSONIndent, "i", false, "Indent / pretty print JSON")
	listFlags.BoolVar(&listCSV, "c", false, "Output CSV")
	listFlags.BoolVar(&helpFlag, "h", false, helpUsage)
}

var listFlags *flag.FlagSet
var listJSON, listJSONIndent bool
var listCSV bool

var lsTabWriter *tabwriter.Writer
var csvWriter *csv.Writer
var jsonEncoder *json.Encoder

func flagsList(command string, args []string) []string {
	listFlags.Parse(args)
	checkHelpFlag(command)
	if listJSON && listCSV {
		logError.Fatalln("only one of -j or -c allowed")
	}
	return listFlags.Args()
}

func commandLS(ct ComponentType, args []string, params []string) (err error) {
	if ct == Remote {
		// geneos ls remote [NAME]
		if len(args) == 0 {
			// list remotes

		}
	}

	switch {
	case listJSON:
		jsonEncoder = json.NewEncoder(log.Writer())
		if listJSONIndent {
			jsonEncoder.SetIndent("", "    ")
		}
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
	logDebug.Println("params", params)
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

type psType struct {
	Type      string
	Name      string
	PID       string
	User      string
	Group     string
	Starttime string
	Home      string
}

func commandPS(ct ComponentType, args []string, params []string) (err error) {
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
	return loopCommand(commandInstance, ct, args, params)
}

func commandInstance(c Instance, params []string) (err error) {
	cmd, env := buildCmd(c)
	if cmd != nil {
		log.Printf("command: %q\n", cmd.String())
		log.Println("env:")
		for _, e := range env {
			log.Println(e)
		}
	}
	return
}
