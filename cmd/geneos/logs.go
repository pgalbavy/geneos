package main

import (
	"bufio"
	"errors"
	"flag"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/fsnotify/fsnotify"
)

func init() {
	commands["logs"] = Command{
		Function:    commandLogs,
		ParseFlags:  logsFlag,
		ParseArgs:   parseArgs,
		CommandLine: "geneos logs [FLAGS] [TYPE] [NAME...]",
		Summary:     `Show log(s) for instances.`,
		Description: `Show log(s) for instances.

FLAGS:
	-n NUM		- show last NUM lines, default 10
	-f		- follow
	-c		- cat log file(s)
	-g STRING	- "grep" STRING (plain, non-regexp)
	-v STRING	- "grep -v" STRING (plain, non-regexp)

-g and -v cannot be combined
-c and -f cannot be combined
-n is ignored when -c is given

When more than one instance matches each output block is prefixed by instance details.
`}

	logsFlags = flag.NewFlagSet("logs", flag.ExitOnError)
	logsFlags.IntVar(&logsLines, "n", 10, "Lines to tail")
	logsFlags.BoolVar(&logsFollow, "f", false, "Follow file")
	logsFlags.BoolVar(&logsCat, "c", false, "Cat whole file")
	logsFlags.StringVar(&logsInclude, "g", "", "Match lines with STRING")
	logsFlags.StringVar(&logsExclude, "v", "", "Match lines without STRING")
	logsFlags.BoolVar(&helpFlag, "h", false, helpUsage)
}

var logsFlags *flag.FlagSet
var logsFollow, logsCat bool
var logsLines int
var logsInclude, logsExclude string

// global watcher for logs
// we need this right now so that logFollowInstance() knows to add files to it
// abstract this away somehow
var watcher *fsnotify.Watcher

// struct to hold logfile details
type tail struct {
	f    io.ReadSeekCloser
	ct   Component
	name string
}

// map of log file path to File set to the last position read
var tails map[string]*tail = make(map[string]*tail)

// last logfile written out
var lastout string

func logsFlag(command string, args []string) []string {
	logsFlags.Parse(args)
	checkHelpFlag(command)
	return logsFlags.Args()
}

func commandLogs(ct Component, args []string, params []string) (err error) {
	// validate options
	if logsInclude != "" && logsExclude != "" {
		logError.Fatalln("Only one of -g or -v can be given")
	}

	if logsCat && logsFollow {
		logError.Fatalln("Only one of -c or -f can be given")
	}

	switch {
	case logsCat:
		return loopCommand(logCatInstance, ct, args, params)
	case logsFollow:
		// tail -f here
		done := make(chan bool)
		watcher, _ = watchLogs()
		defer watcher.Close()
		// add logs to watcher
		// wait for events
		// track end of each file
		// support rolling via Rename / Create events
		err = loopCommand(logFollowInstance, ct, args, params)

		<-done
	default:
		err = loopCommand(logTailInstance, ct, args, params)
		// just tail a number of lines
	}

	return
}

func outHeader(logfile string) {
	if lastout == logfile {
		return
	}
	if lastout != "" {
		log.Println()
	}
	log.Printf("==> %s:%s %s <==\n", tails[logfile].ct, tails[logfile].name, logfile)
	lastout = logfile
}

func logTailInstance(c Instances, params []string) (err error) {
	logfile := getLogfilePath(c)

	lines, st, err := statAndOpenFile(c.Location(), logfile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return
	}
	defer lines.Close()
	tails[logfile] = &tail{lines, c.Type(), c.Name() + "@" + c.Location()}

	text, err := tailLines(lines, st, logsLines)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println(err)
	}
	if len(text) != 0 {
		filterOutput(logfile, strings.NewReader(text+"\n"))
	}
	return nil
}

func tailLines(f io.ReadSeekCloser, st fileStat, linecount int) (text string, err error) {
	// reasonable guess at bytes per line to use as a multiplier
	const charsPerLine = 132
	var chunk int64 = int64(linecount * charsPerLine)
	var buf []byte = make([]byte, chunk)
	var i int64
	var alllines []string = []string{""}

	if f == nil {
		return
	}
	if linecount == 0 {
		// seek to end and return
		_, err = f.Seek(0, os.SEEK_END)
		return
	}

	// st, err := f.Stat()
	if err != nil {
		return
	}
	var end int64
	if st != (fileStat{}) {
		end = st.st.Size()
	}

	for i = 1 + end/chunk; i > 0; i-- {
		f.Seek((i-1)*chunk, io.SeekStart)
		n, err := f.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			logError.Fatalln(err)
		}
		strbuf := string(buf[:n])

		// split buffer, count lines, if enough shortcut a return
		// else keep alllines[0] (partial end of previous line), save the rest and
		// repeat until beginning of file or N lines
		newlines := strings.FieldsFunc(strbuf+alllines[0], isLineSep)
		alllines = append(newlines, alllines[1:]...)
		if len(alllines) > linecount {
			text = strings.Join(alllines[len(alllines)-linecount:], "\n")
			f.Seek(end, io.SeekStart)
			return text, err
		}
	}

	text = strings.Join(alllines, "\n")
	f.Seek(end, io.SeekStart)
	return
}

func isLineSep(r rune) bool {
	if r == rune('\n') || r == rune('\r') {
		return true
	}
	return unicode.Is(unicode.Zp, r)
}

func filterOutput(logfile string, reader io.Reader) {
	switch {
	case logsInclude != "":
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, logsInclude) {
				outHeader(logfile)
				log.Println(line)
			}
		}
	case logsExclude != "":
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.Contains(line, logsExclude) {
				outHeader(logfile)
				log.Println(line)
			}
		}
	default:
		outHeader(logfile)
		if _, err := io.Copy(log.Writer(), reader); err != nil {
			log.Println(err)
		}
		//log.Println()
	}
}

func logCatInstance(c Instances, params []string) (err error) {
	logfile := getLogfilePath(c)

	lines, _, err := statAndOpenFile(c.Location(), logfile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return
	}
	tails[logfile] = &tail{lines, c.Type(), c.Name() + "@" + c.Location()}
	defer lines.Close()
	filterOutput(logfile, lines)

	return
}

func logFollowInstance(c Instances, params []string) (err error) {
	if c.Location() != LOCAL {
		log.Printf("===> %s %s@%s -f not supported for remote instances, ignoring <===", c.Type(), c.Name(), c.Location())
		return
	}
	logfile := getLogfilePath(c)

	f, st, err := statAndOpenFile(LOCAL, logfile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return
	}
	// perfectly valid to not have a file to watch at start
	tails[logfile] = &tail{f, c.Type(), c.Name() + "@" + c.Location()}

	// output up to this point
	text, _ := tailLines(tails[logfile].f, st, logsLines)

	if len(text) != 0 {
		filterOutput(logfile, strings.NewReader(text+"\n"))
	}

	logDebug.Println("watching", logfile)

	// add parent directory, to watch for changes
	// no worries about tidying up, process is short lived
	if err = watcher.Add(filepath.Dir(logfile)); err != nil {
		logDebug.Fatalln("watcher.Add():", logfile, err)
	}

	return
}

func watchLogs() (watcher *fsnotify.Watcher, err error) {
	// set up watcher
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		logError.Fatal(err)
	}

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				logDebug.Println("event:", event)
				// check directory changes too
				if tail, ok := tails[event.Name]; ok {
					switch {
					case event.Op&fsnotify.Create > 0:
						logDebug.Println("create", event.Name)
						if tail.f != nil {
							tail.f.Close()
						}
						if tail.f, err = os.Open(event.Name); err != nil {
							log.Println("cannot (re)open", err)
						}
					case event.Op&fsnotify.Write > 0:
						logDebug.Println("modified file:", event.Name)
						copyFromFile(event.Name)
					case event.Op&fsnotify.Rename > 0, event.Op&fsnotify.Remove > 0:
						logDebug.Println("rename/remove", event.Name)
						tail.f.Close()
						tail.f = nil
					}
				}

			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	return
}

func copyFromFile(logfile string) {
	if tail, ok := tails[logfile]; ok {
		if tail.f != nil {
			logDebug.Println("tail", logfile)
			filterOutput(logfile, tail.f)
		}
	}
}
