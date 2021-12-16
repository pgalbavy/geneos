package main

import (
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/fsnotify/fsnotify"
)

var logsFlags *flag.FlagSet
var logsFollow, logsCat bool
var logsLines int
var logsInclude, logsExclude string

func init() {
	logsFlags = flag.NewFlagSet("logs", flag.ExitOnError)
	logsFlags.IntVar(&logsLines, "n", 20, "Lines to tail")
	logsFlags.BoolVar(&logsFollow, "f", false, "Follow file")
	logsFlags.BoolVar(&logsCat, "c", false, "Cat whole file")
	logsFlags.StringVar(&logsInclude, "g", "", "Filter output with STRING")
	logsFlags.StringVar(&logsExclude, "v", "", "Filter output with STRING")

	commands["logs"] = Command{commandLogs, parseArgs, "geneos logs [TYPE] [NAME...]",
		`Show logs for matching instances. Not fully implemented.

Options:
	-n NUM		- show last NUM lines, default 20
	-f		- follow
	-c		- cat log file(s)
	-g STRING	- "grep" STRING (plain, non-regexp)
	-v STRING	- "grep -v" STRING (plain, non-regexp)

-g and -v cannot be combined
-c and -f cannot be combined and -n is ignored

When one instance given just stream, otherwise each output block is prefixed by instance details.
`}
}

var watcher *fsnotify.Watcher

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

func logTailInstance(c Instance, params []string) (err error) {
	logfile := getLogfilePath(c)

	log.Println("**** log for", Type(c), Name(c), "****")
	lines, err := os.Open(logfile)
	if err != nil {
		return
	}
	defer lines.Close()

	text, err := tailLines(lines, logsLines)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println(err)
	}
	log.Print(text)
	return nil
}

// given a seekable file, go to the end and read blocks until
// the start iof the N-the line from the end. Stop if we reach the start of
// the file. Return the string from the start of that line to the end of the
// file (end when passed in, growing file is allowed)
//
// support unicode
func tailLines(file *os.File, linecount int) (text string, err error) {
	// reasonable guess at bytes per line to use as a multiplier
	const charsPerLine = 132
	var chunk int64 = int64(linecount * charsPerLine)
	var buf []byte = make([]byte, chunk)

	// save current end of file
	// end, err := file.Seek(0, io.SeekEnd)
	st, _ := file.Stat()
	end := st.Size()

	var i int64
	var alllines []string = []string{""}
	for i = 1 + end/chunk; i > 0; i-- {
		n, err := file.ReadAt(buf, (i-1)*chunk)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Fatalln(err)
		}
		strbuf := string(buf[:n])

		// split buffer, count alllines, if enough shortcut a return
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

func logCatInstance(c Instance, params []string) (err error) {
	logfile := getLogfilePath(c)

	log.Println("**** log for", Type(c), Name(c), "****")
	lines, err := os.Open(logfile)
	if err != nil {
		return
	}
	defer lines.Close()
	_, err = io.Copy(log.Writer(), lines)
	return
}

func logFollowInstance(c Instance, params []string) (err error) {
	logfile := getLogfilePath(c)

	DebugLog.Println("watching", logfile)
	if err = watcher.Add(logfile); err != nil {
		log.Fatalln("watcher.Add():", logfile, err)
	}

	return
}

func watchLogs() (err error) {
	// set up watcher
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	//defer watcher.Close()

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
					copyFromFile()
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	return
}

func copyFromFile() {

}
