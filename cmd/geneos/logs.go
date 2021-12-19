package main

import (
	"bufio"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/fsnotify/fsnotify"
)

func init() {
	commands["logs"] = Command{commandLogs, parseArgs, "geneos logs [TYPE] [NAME...]",
		`Show logs for matching instances. Not fully implemented.

Options:
	-n NUM		- show last NUM lines, default 10
	-f		- follow
	-c		- cat log file(s)
	-g STRING	- "grep" STRING (plain, non-regexp)
	-v STRING	- "grep -v" STRING (plain, non-regexp)

-g and -v cannot be combined
-c and -f cannot be combined and -n is ignored

When one instance given just stream, otherwise each output block is prefixed by instance details.
`}

	logsFlags = flag.NewFlagSet("logs", flag.ExitOnError)
	logsFlags.IntVar(&logsLines, "n", 10, "Lines to tail")
	logsFlags.BoolVar(&logsFollow, "f", false, "Follow file")
	logsFlags.BoolVar(&logsCat, "c", false, "Cat whole file")
	logsFlags.StringVar(&logsInclude, "g", "", "Filter output with STRING")
	logsFlags.StringVar(&logsExclude, "v", "", "Filter output with STRING")

}

var logsFlags *flag.FlagSet
var logsFollow, logsCat bool
var logsLines int
var logsInclude, logsExclude string

var watcher *fsnotify.Watcher

// struct to hold logfile details
type tail struct {
	f *os.File
	t ComponentType
	n string
}

// map of log file path to File set to the last position read
var tails map[string]*tail = make(map[string]*tail)

// last logfile written out
var lastout string

func commandLogs(ct ComponentType, args []string, params []string) (err error) {
	logsFlags.Parse(params)
	params = logsFlags.Args()

	// validate options
	if logsInclude != "" && logsExclude != "" {
		log.Fatalln("Only one of -g or -v can be given")
	}

	if logsCat && logsFollow {
		log.Fatalln("Only one of -c or -f can be given")
	}

	switch {
	case logsCat:
		return loopCommand(logCatInstance, ct, args, params)
	case logsFollow:
		// tail -f here
		done := make(chan bool)
		watchLogs()
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
	log.Printf("==> %s:%s %s <==\n", tails[logfile].t, tails[logfile].n, logfile)
	lastout = logfile
}

func logTailInstance(c Instance, params []string) (err error) {
	logfile := getLogfilePath(c)

	lines, err := os.Open(logfile)
	if err != nil {
		return
	}
	defer lines.Close()
	tails[logfile] = &tail{lines, Type(c), Name(c)}

	text, err := tailLines(lines, logsLines)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println(err)
	}
	if len(text) != 0 {
		filterOutput(logfile, strings.NewReader(text+"\n"))
	}
	return nil
}

func tailLines(file *os.File, linecount int) (text string, err error) {
	// reasonable guess at bytes per line to use as a multiplier
	const charsPerLine = 132
	var chunk int64 = int64(linecount * charsPerLine)
	var buf []byte = make([]byte, chunk)
	var i int64
	var alllines []string = []string{""}

	if linecount == 0 {
		// seek to end and return
		_, err = file.Seek(0, os.SEEK_END)
		return
	}

	st, _ := file.Stat()
	end := st.Size()

	for i = 1 + end/chunk; i > 0; i-- {
		n, err := file.ReadAt(buf, (i-1)*chunk)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Fatalln(err)
		}
		strbuf := string(buf[:n])

		// split buffer, count lines, if enough shortcut a return
		// else keep alllines[0] (partial end of previous line), save the rest and
		// repeat until beginning of file or N lines
		newlines := strings.FieldsFunc(strbuf+alllines[0], isLineSep)
		alllines = append(newlines, alllines[1:]...)
		if len(alllines) > linecount {
			text = strings.Join(alllines[len(alllines)-linecount:], "\n")
			file.Seek(end, io.SeekStart)
			return text, err
		}
	}

	text = strings.Join(alllines, "\n")
	file.Seek(end, io.SeekStart)
	return
}

func isLineSep(r rune) bool {
	//DebugLog.Println(r, string(r))
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

func logCatInstance(c Instance, params []string) (err error) {
	logfile := getLogfilePath(c)

	lines, err := os.Open(logfile)
	if err != nil {
		return
	}
	tails[logfile] = &tail{lines, Type(c), Name(c)}
	defer lines.Close()
	filterOutput(logfile, lines)

	return
}

func logFollowInstance(c Instance, params []string) (err error) {
	logfile := getLogfilePath(c)

	f, err := os.Open(logfile)
	if err != nil {
		return
	}
	tails[logfile] = &tail{f, Type(c), Name(c)}

	// output up to this point
	text, err := tailLines(tails[logfile].f, logsLines)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println(err)
	}
	if len(text) != 0 {
		filterOutput(logfile, strings.NewReader(text+"\n"))
	}

	DebugLog.Println("watching", logfile)

	// add parent directory, to watch for changes
	// no worries about tidying up, process is short lived
	if err = watcher.Add(logfile); err != nil {
		DebugLog.Fatalln("watcher.Add():", logfile, err)
	}
	if err = watcher.Add(filepath.Dir(logfile)); err != nil {
		DebugLog.Fatalln("watcher.Add():", logfile, err)
	}

	return
}

func watchLogs() (err error) {
	// set up watcher
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				DebugLog.Println("event:", event)
				// check directory changes too
				if tail, ok := tails[event.Name]; ok {
					switch {
					case event.Op&fsnotify.Create > 0:
						DebugLog.Println("create", event.Name)
						if tail.f != nil {
							tail.f.Close()
						}
						if tail.f, err = os.Open(event.Name); err != nil {
							log.Println("cannot re-open", err)
						}
						if err = watcher.Add(event.Name); err != nil {
							log.Println("cannot watch", err)
						}
						DebugLog.Println("watching", event.Name)
					case event.Op&fsnotify.Write > 0:
						DebugLog.Println("modified file:", event.Name)
						copyFromFile(event.Name)
					case event.Op&fsnotify.Rename > 0, event.Op&fsnotify.Remove > 0:
						DebugLog.Println("rename/remove", event.Name)
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
			DebugLog.Println("tail", logfile)
			filterOutput(logfile, tail.f)
		}
	}
}
