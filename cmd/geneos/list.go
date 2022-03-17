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
		ParseArgs:   defaultArgs,
		CommandLine: "geneos ls [-c|-j] [TYPE] [NAME...]",
		Summary:     `List instances, optionally in CSV or JSON format.`,
		Description: `List the matching instances and their component type.
	
Future versions will support CSV or JSON output formats for automation and monitoring.`}

	commands["ps"] = Command{
		Function:    commandPS,
		ParseFlags:  flagsList,
		ParseArgs:   defaultArgs,
		CommandLine: "geneos ps [-c|-j] [TYPE] [NAMES...]",
		Summary:     `List process information for instances, optionally in CSV or JSON format.`,
		Description: `Show the status of the matching instances.

Future versions will support CSV or JSON output formats for automation and monitoring.`}

	commands["command"] = Command{
		Function:    commandCommand,
		ParseFlags:  nil,
		ParseArgs:   defaultArgs,
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

func commandLS(ct Component, args []string, params []string) (err error) {
	if ct == Remote {
		switch {
		case listJSON:
			jsonEncoder = json.NewEncoder(log.Writer())
			if listJSONIndent {
				jsonEncoder.SetIndent("", "    ")
			}
			err = ct.loopCommand(lsInstanceJSONRemotes, args, params)
		case listCSV:
			csvWriter = csv.NewWriter(log.Writer())
			csvWriter.Write([]string{"Type", "Name", "Disabled", "Hostname", "Port", "ITRSHome"})
			err = ct.loopCommand(lsInstanceCSVRemotes, args, params)
			csvWriter.Flush()
		default:
			lsTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
			fmt.Fprintf(lsTabWriter, "Type\tName\tHostname:Port\tITRSHome\n")
			err = ct.loopCommand(lsInstancePlainRemotes, args, params)
			lsTabWriter.Flush()
		}
		return
	}

	switch {
	case listJSON:
		jsonEncoder = json.NewEncoder(log.Writer())
		if listJSONIndent {
			jsonEncoder.SetIndent("", "    ")
		}
		err = ct.loopCommand(lsInstanceJSON, args, params)
	case listCSV:
		csvWriter = csv.NewWriter(log.Writer())
		csvWriter.Write([]string{"Type", "Name", "Disabled", "Location", "Home"})
		err = ct.loopCommand(lsInstanceCSV, args, params)
		csvWriter.Flush()
	default:
		lsTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
		fmt.Fprintf(lsTabWriter, "Type\tName\tLocation\tHome\n")
		err = ct.loopCommand(lsInstancePlain, args, params)
		lsTabWriter.Flush()
	}
	return
}

func lsInstancePlain(c Instances, params []string) (err error) {
	var suffix string
	if Disabled(c) {
		suffix = "*"
	}
	fmt.Fprintf(lsTabWriter, "%s\t%s\t%s\t%s\n", c.Type(), c.Name()+suffix, c.Location(), c.Home())
	return
}

func lsInstancePlainRemotes(c Instances, params []string) (err error) {
	var suffix string
	if Disabled(c) {
		suffix = "*"
	}
	fmt.Fprintf(lsTabWriter, "%s\t%s\t%s:%d\t%s\n", c.Type(), c.Name()+suffix, getString(c, "Hostname"), getInt(c, "Port"), getString(c, "ITRSHome"))
	return
}

func lsInstanceCSV(c Instances, params []string) (err error) {
	var dis string = "N"
	if Disabled(c) {
		dis = "Y"
	}
	csvWriter.Write([]string{c.Type().String(), c.Name(), dis, string(c.Location()), c.Home()})
	return
}

func lsInstanceCSVRemotes(c Instances, params []string) (err error) {
	var dis string = "N"
	if Disabled(c) {
		dis = "Y"
	}
	csvWriter.Write([]string{c.Type().String(), c.Name(), dis, getString(c, "Hostname"), fmt.Sprint(getInt(c, "Port")), getString(c, "ITRSHome")})
	return
}

type lsType struct {
	Type     string
	Name     string
	Disabled string
	Location string
	Home     string
}

type lsTypeRemotes struct {
	Type     string
	Name     string
	Disabled string
	Hostname string
	Port     int64
	ITRSHome string
}

func lsInstanceJSON(c Instances, params []string) (err error) {
	var dis string = "N"
	if Disabled(c) {
		dis = "Y"
	}
	jsonEncoder.Encode(lsType{c.Type().String(), c.Name(), dis, string(c.Location()), c.Home()})
	return
}

func lsInstanceJSONRemotes(c Instances, params []string) (err error) {
	var dis string = "N"
	if Disabled(c) {
		dis = "Y"
	}
	jsonEncoder.Encode(lsTypeRemotes{c.Type().String(), c.Name(), dis, getString(c, "Hostname"), getInt(c, "Port"), getString(c, "ITRSHome")})
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
	Remote    string
	PID       string
	User      string
	Group     string
	Starttime string
	Home      string
}

func commandPS(ct Component, args []string, params []string) (err error) {
	switch {
	case listJSON:
		jsonEncoder = json.NewEncoder(log.Writer())
		//jsonEncoder.SetIndent("", "    ")
		err = ct.loopCommand(psInstanceJSON, args, params)
	case listCSV:
		csvWriter = csv.NewWriter(log.Writer())
		csvWriter.Write([]string{"Type:Name@Location", "PID", "User", "Group", "Starttime", "Home"})
		err = ct.loopCommand(psInstanceCSV, args, params)
		csvWriter.Flush()
	default:
		psTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
		fmt.Fprintf(psTabWriter, "Type:Name@Location\tPID\tUser\tGroup\tStarttime\tHome\n")
		err = ct.loopCommand(psInstancePlain, args, params)
		psTabWriter.Flush()
	}
	return
}

func psInstancePlain(c Instances, params []string) (err error) {
	if Disabled(c) {
		return nil
	}
	pid, uid, gid, mtime, err := findInstanceProc(c)
	if err != nil {
		return nil
	}

	var u *user.User
	var g *user.Group

	username := fmt.Sprint(uid)
	groupname := fmt.Sprint(gid)

	if u, err = user.LookupId(username); err == nil {
		username = u.Username
	}
	if g, err = user.LookupGroupId(groupname); err == nil {
		groupname = g.Name
	}

	fmt.Fprintf(psTabWriter, "%s:%s@%s\t%d\t%s\t%s\t%s\t%s\n", c.Type(), c.Name(), c.Location(), pid, username, groupname, time.Unix(mtime, 0).Local().Format(time.RFC3339), c.Home())

	return
}

func psInstanceCSV(c Instances, params []string) (err error) {
	if Disabled(c) {
		return nil
	}
	pid, uid, gid, mtime, err := findInstanceProc(c)
	if err != nil {
		return nil
	}

	var u *user.User
	var g *user.Group

	username := fmt.Sprint(uid)
	groupname := fmt.Sprint(gid)

	if u, err = user.LookupId(username); err == nil {
		username = u.Username
	}
	if g, err = user.LookupGroupId(groupname); err == nil {
		groupname = g.Name
	}

	csvWriter.Write([]string{c.Type().String() + ":" + c.Name() + "@" + string(c.Location()), fmt.Sprint(pid), username, groupname, time.Unix(mtime, 0).Local().Format(time.RFC3339), c.Home()})

	return
}

func psInstanceJSON(c Instances, params []string) (err error) {
	if Disabled(c) {
		return nil
	}
	pid, uid, gid, mtime, err := findInstanceProc(c)
	if err != nil {
		return nil
	}

	var u *user.User
	var g *user.Group

	username := fmt.Sprint(uid)
	groupname := fmt.Sprint(gid)

	if u, err = user.LookupId(username); err == nil {
		username = u.Username
	}
	if g, err = user.LookupGroupId(groupname); err == nil {
		groupname = g.Name
	}

	jsonEncoder.Encode(psType{c.Type().String(), c.Name(), string(c.Location()), fmt.Sprint(pid), username, groupname, time.Unix(mtime, 0).Local().Format(time.RFC3339), c.Home()})

	return
}

func commandCommand(ct Component, args []string, params []string) (err error) {
	return ct.loopCommand(commandInstance, args, params)
}

func commandInstance(c Instances, params []string) (err error) {
	log.Printf("=== %s %s@%s ===", c.Type(), c.Name(), c.Location())
	cmd, env := buildCmd(c)
	if cmd != nil {
		log.Println("command line:")
		log.Println("\t", cmd.String())
		log.Println()
		log.Println("environment:")
		for _, e := range env {
			log.Println("\t", e)
		}
		log.Println()
	}
	return
}
