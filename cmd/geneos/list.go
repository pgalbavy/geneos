package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os/user"
	"path/filepath"
	"text/tabwriter"
	"time"
)

func init() {
	RegsiterCommand(Command{
		Name:          "ls",
		Function:      commandLS,
		ParseFlags:    listFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos ls [-c|-j [-i]] [TYPE] [NAME...]",
		Summary:       `List instances, optionally in CSV or JSON format.`,
		Description: `List the matching instances and their component type.

FLAGS:
	-c	Output CSV format
	-j	Output JSON format
	-i	Indent (pretty print) JSON output
`,
	})

	RegsiterCommand(Command{
		Name:          "ps",
		Function:      commandPS,
		ParseFlags:    listFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos ps [-c|-j [-i]] [TYPE] [NAMES...]",
		Summary:       `List process information for instances, optionally in CSV or JSON format.`,
		Description: `Show the status of the matching instances.

FLAGS:
		-c	Output CSV format
		-j	Output JSON format
		-i	Indent (pretty print) JSON output
	`,
	})

	RegsiterCommand(Command{
		Name:          "command",
		Function:      commandCommand,
		ParseFlags:    nil,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos command [TYPE] [NAME...]",
		Summary:       `Show command arguments and environment for instances.`,
		Description: `Show the full command line for the matching instances along with any environment variables
explicitly set for execution.

Future releases may support CSV or JSON output formats for automation and monitoring.`,
	})

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

func listFlag(command string, args []string) []string {
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
			csvWriter.Write([]string{"Type", "Name", "Disabled", "Username", "Hostname", "Port", "Geneos"})
			err = ct.loopCommand(lsInstanceCSVRemotes, args, params)
			csvWriter.Flush()
		default:
			lsTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
			fmt.Fprintf(lsTabWriter, "Type\tName\tUsername\tHostname\tPort\tITRSHome\n")
			err = ct.loopCommand(lsInstancePlainRemotes, args, params)
			lsTabWriter.Flush()
		}
		if err == ErrNotFound {
			err = nil
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
		csvWriter.Write([]string{"Type", "Name", "Disabled", "Location", "Port", "Version", "Home"})
		err = ct.loopCommand(lsInstanceCSV, args, params)
		csvWriter.Flush()
	default:
		lsTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
		fmt.Fprintf(lsTabWriter, "Type\tName\tLocation\tPort\tVersion\tHome\n")
		err = ct.loopCommand(lsInstancePlain, args, params)
		lsTabWriter.Flush()
	}
	if err == ErrNotFound {
		err = nil
	}
	return
}

func lsInstancePlain(c Instances, params []string) (err error) {
	var suffix string
	if Disabled(c) {
		suffix = "*"
	}
	base, underlying, _ := componentVersion(c)
	fmt.Fprintf(lsTabWriter, "%s\t%s\t%s\t%d\t%s:%s\t%s\n", c.Type(), c.Name()+suffix, c.Location(), getInt(c, c.Prefix("Port")), base, underlying, c.Home())
	return
}

func lsInstancePlainRemotes(c Instances, params []string) (err error) {
	var suffix string
	if Disabled(c) {
		suffix = "*"
	}
	fmt.Fprintf(lsTabWriter, "%s\t%s\t%s\t%s\t%d\t%s\n", c.Type(), c.Name()+suffix, getString(c, "Username"), getString(c, "Hostname"), getInt(c, "Port"), getString(c, "Geneos"))
	return
}

func lsInstanceCSV(c Instances, params []string) (err error) {
	var dis string = "N"
	if Disabled(c) {
		dis = "Y"
	}
	base, underlying, _ := componentVersion(c)
	csvWriter.Write([]string{c.Type().String(), c.Name(), dis, string(c.Location()), fmt.Sprint(getInt(c, c.Prefix("Port"))), fmt.Sprintf("%s:%s", base, underlying), c.Home()})
	return
}

func lsInstanceCSVRemotes(c Instances, params []string) (err error) {
	var dis string = "N"
	if Disabled(c) {
		dis = "Y"
	}
	csvWriter.Write([]string{c.Type().String(), c.Name(), dis, getString(c, "Username"), getString(c, "Hostname"), fmt.Sprint(getInt(c, "Port")), getString(c, "Geneos")})
	return
}

type lsType struct {
	Type     string
	Name     string
	Disabled string
	Location string
	Port     int64
	Version  string
	Home     string
}

type lsTypeRemotes struct {
	Type     string
	Name     string
	Disabled string
	Username string
	Hostname string
	Port     int64
	Geneos   string
}

func lsInstanceJSON(c Instances, params []string) (err error) {
	var dis string = "N"
	if Disabled(c) {
		dis = "Y"
	}
	base, underlying, _ := componentVersion(c)
	jsonEncoder.Encode(lsType{c.Type().String(), c.Name(), dis, string(c.Location()), getInt(c, c.Prefix("Port")), fmt.Sprintf("%s:%s", base, underlying), c.Home()})
	return
}

func lsInstanceJSONRemotes(c Instances, params []string) (err error) {
	var dis string = "N"
	if Disabled(c) {
		dis = "Y"
	}
	jsonEncoder.Encode(lsTypeRemotes{c.Type().String(), c.Name(), dis, getString(c, "Username"), getString(c, "Hostname"), getInt(c, "Port"), getString(c, "Geneos")})
	return
}

var psTabWriter *tabwriter.Writer

type psType struct {
	Type      string
	Name      string
	Remote    string
	PID       string
	User      string
	Group     string
	Starttime string
	Version   string
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
		csvWriter.Write([]string{"Type", "Name", "Location", "PID", "User", "Group", "Starttime", "Version", "Home"})
		err = ct.loopCommand(psInstanceCSV, args, params)
		csvWriter.Flush()
	default:
		psTabWriter = tabwriter.NewWriter(log.Writer(), 3, 8, 2, ' ', 0)
		fmt.Fprintf(psTabWriter, "Type\tName\tLocation\tPID\tUser\tGroup\tStarttime\tVersion\tHome\n")
		err = ct.loopCommand(psInstancePlain, args, params)
		psTabWriter.Flush()
	}
	if err == ErrNotFound {
		err = nil
	}
	return
}

func psInstancePlain(c Instances, params []string) (err error) {
	if Disabled(c) {
		return nil
	}
	pid, uid, gid, mtime, err := findInstancePIDInfo(c)
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
	base, underlying, _ := componentVersion(c)
	fmt.Fprintf(psTabWriter, "%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s:%s\t%s\n", c.Type(), c.Name(), c.Location(), pid, username, groupname, time.Unix(mtime, 0).Local().Format(time.RFC3339), base, underlying, c.Home())

	return
}

func psInstanceCSV(c Instances, params []string) (err error) {
	if Disabled(c) {
		return nil
	}
	pid, uid, gid, mtime, err := findInstancePIDInfo(c)
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
	base, underlying, _ := componentVersion(c)
	csvWriter.Write([]string{c.Type().String(), c.Name(), c.Location().String(), fmt.Sprint(pid), username, groupname, time.Unix(mtime, 0).Local().Format(time.RFC3339), fmt.Sprintf("%s:%s", base, underlying), c.Home()})

	return
}

func psInstanceJSON(c Instances, params []string) (err error) {
	if Disabled(c) {
		return nil
	}
	pid, uid, gid, mtime, err := findInstancePIDInfo(c)
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
	base, underlying, _ := componentVersion(c)
	jsonEncoder.Encode(psType{c.Type().String(), c.Name(), string(c.Location()), fmt.Sprint(pid), username, groupname, time.Unix(mtime, 0).Local().Format(time.RFC3339), fmt.Sprintf("%s:%s", base, underlying), c.Home()})

	return
}

func commandCommand(ct Component, args []string, params []string) (err error) {
	return ct.loopCommand(commandInstance, args, params)
}

func commandInstance(c Instances, params []string) (err error) {
	log.Printf("=== %s ===", c)
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

//
// return the base package name and the version it links to.
// if not a link, then return the same
// follow a limited number of links (10?)
func componentVersion(c Instances) (base string, underlying string, err error) {
	basedir := getString(c, c.Prefix("Bins"))
	base = getString(c, c.Prefix("Base"))
	underlying = base
	for {
		basepath := filepath.Join(basedir, underlying)
		var st fileStat
		st, err = c.Remote().lstatFile(basepath)
		if err != nil {
			underlying = "unknown"
			return
		}
		if st.st.Mode()&fs.ModeSymlink != 0 {
			underlying, err = c.Remote().readlink(basepath)
			if err != nil {
				underlying = "unknown"
				return
			}
		} else {
			break
		}
	}
	return
}
