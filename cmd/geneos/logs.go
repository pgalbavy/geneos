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
	"sync"
	"time"
	"unicode"

	"github.com/fsnotify/fsnotify"
)

func init() {
	RegsiterCommand(Command{
		Name:          "logs",
		Function:      commandLogs,
		ParseFlags:    logsFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos logs [FLAGS] [TYPE] [NAME...]",
		Summary:       `Show log(s) for instances.`,
		Description: `Show log(s) for instances. The default is to show the last 10 lines
for each matching instance. If either -g or -v are given without -f
to follow live logs, then -c to search the whole log is implied.

Follow (-f) only works for local log files and not for remote instances.

FLAGS:
	-n NUM		- show last NUM lines, default 10
	-f		- follow in real time
	-c		- 'cat' whole log file(s)
	-g STRING	- "grep" STRING (plain, non-regexp)
	-v STRING	- "grep -v" STRING (plain, non-regexp)

-g and -v cannot be combined
-c and -f cannot be combined
-n is ignored when -c is given

When more than one instance matches each output block is prefixed by instance details.`,
	})

	logsFlags = flag.NewFlagSet("logs", flag.ExitOnError)
	logsFlags.IntVar(&logsLines, "n", 10, "Lines to tail")
	logsFlags.BoolVar(&logsFollow, "f", false, "Follow file")
	logsFlags.BoolVar(&logsCat, "c", false, "Cat whole file")
	logsFlags.StringVar(&logsMatchLines, "g", "", "Match lines with STRING")
	logsFlags.StringVar(&logsExclude, "v", "", "Match lines without STRING")
	logsFlags.BoolVar(&helpFlag, "h", false, helpUsage)
}

var logsFlags *flag.FlagSet
var logsFollow, logsCat bool
var logsLines int
var logsMatchLines, logsExclude string

// global watchers for logs
// we need this right now so that logFollowInstance() knows to add files to it
// abstract this away somehow
var watcherLocal *fsnotify.Watcher
var watcherRemote *sync.Map

// struct to hold logfile details
type tail struct {
	f io.ReadSeekCloser
	//ct   Component
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
	if logsMatchLines != "" && logsExclude != "" {
		logError.Fatalln("Only one of -g or -v can be given")
	}

	if logsCat && logsFollow {
		logError.Fatalln("Only one of -c or -f can be given")
	}

	// if we have match or exclude with other defaults, then turn on logcat
	if (logsMatchLines != "" || logsExclude != "") && !logsFollow {
		logsCat = true
	}

	switch {
	case logsCat:
		err = ct.loopCommand(logCatInstance, args, params)
	case logsFollow:
		// never returns
		err = ct.followLogs(args, params)
	default:
		err = ct.loopCommand(logTailInstance, args, params)
	}

	return
}

func (ct Component) followLogs(args, params []string) (err error) {
	done := make(chan bool)
	watcherLocal, _ = watchLocalLogs()
	watcherRemote, _ = watchRemoteLogs()
	defer watcherLocal.Close()
	if err = ct.loopCommand(logFollowInstance, args, params); err != nil {
		log.Println(err)
	}
	<-done
	return
}

func outHeader(logfile string) {
	if lastout == logfile {
		return
	}
	if lastout != "" {
		log.Println()
	}
	log.Printf("==> %s %s <==\n", tails[logfile].name, logfile)
	lastout = logfile
}

func logTailInstance(c Instances, params []string) (err error) {
	logfile := getLogfilePath(c)

	f, st, err := c.Remote().statAndOpenFile(logfile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("===> %s log file not found <===", c)
			return nil
		}
		return
	}
	defer f.Close()
	tails[logfile] = &tail{f, c.String()}

	text, err := tailLines(f, st.st.Size(), logsLines)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println(err)
	}
	if len(text) != 0 {
		filterOutput(logfile, strings.NewReader(text+"\n"))
	}
	return nil
}

func tailLines(f io.ReadSeekCloser, end int64, linecount int) (text string, err error) {
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
	case logsMatchLines != "":
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, logsMatchLines) {
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

	lines, _, err := c.Remote().statAndOpenFile(logfile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("===> %s log file not found <===", c)
			return nil
		}
		return
	}
	tails[logfile] = &tail{lines, c.String()}
	defer lines.Close()
	filterOutput(logfile, lines)

	return
}

// add local logs to a watcher list
// for remote logs, spawn a go routine for each log, watch using stat etc.
// and output changes
func logFollowInstance(c Instances, params []string) (err error) {
	logfile := getLogfilePath(c)

	f, st, err := c.Remote().statAndOpenFile(logfile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return
		}
		log.Printf("===> %s log file not found <===", c)
	}

	// perfectly valid to not have a file to watch at start
	tails[logfile] = &tail{f, c.String()}

	if err == nil {
		// output up to this point
		text, _ := tailLines(tails[logfile].f, st.st.Size(), logsLines)

		if len(text) != 0 {
			filterOutput(logfile, strings.NewReader(text+"\n"))
		}
	}
	logDebug.Println("watching", logfile)

	if c.Remote() == rLOCAL {
		// add parent directory, to watch for changes
		// no worries about tidying up, process is short lived
		if err = watcherLocal.Add(filepath.Dir(logfile)); err != nil {
			logError.Fatalln(logfile, err)
		}
		return
	}

	watchRemoteLog(c, watcherRemote)
	return nil
}

// set-up remote watchers
func watchRemoteLogs() (m *sync.Map, err error) {
	m = new(sync.Map)
	ticker := time.NewTicker(500 * time.Millisecond)

	go func() {
		for range ticker.C {
			m.Range(func(key, value interface{}) bool {
				if value == nil {
					return true
				}
				c := key.(Instances)
				oldsize := value.(int64)
				logfile := getLogfilePath(c)
				tail := tails[logfile]

				st, err := c.Remote().statFile(logfile)
				if err != nil {
					return true
				}
				newsize := st.st.Size()

				if newsize == oldsize {
					return true
				}

				// if we have an existing file and it appears
				// to have grown then output whatever is new
				if tail.f != nil {
					tail.f.Seek(oldsize, io.SeekStart)
					copyFromFile(logfile)
					if newsize > oldsize {
						// this seems reqired
						m.Store(key, newsize)
						return true
					}

					// if the file seems to have shrunk, then
					// we are here, so close the old one
					tail.f.Close()
				}

				// open new file, read to the end, return
				if tail.f, st, err = c.Remote().statAndOpenFile(logfile); err != nil {
					log.Println("cannot (re)open", err)
				}
				copyFromFile(logfile)
				m.Store(key, st.st.Size())
				return true
			})
		}
	}()

	return
}

// add a remote instance 'c' log watcher to sync Map 'm'
func watchRemoteLog(c Instances, m *sync.Map) {
	var sz int64
	st, err := c.Remote().statFile(getLogfilePath(c))
	if err == nil {
		sz = st.st.Size()
	}
	m.Store(c, sz)
}

func watchLocalLogs() (watcher *fsnotify.Watcher, err error) {
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
