package logger // import "wonderland.org/geneos/pkg/logger"

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime" // placeholder
	"time"
)

type GeneosLogger struct {
	Writer     io.Writer
	Level      Level
	ShowPrefix bool
}

// debuglog must be defined so it can be set in EnableDebugLog()
// so for consistency do the same for all three loggers
var (
	Logger      = GeneosLogger{os.Stdout, INFO, false}
	DebugLogger = GeneosLogger{os.Stderr, DEBUG, true}
	ErrorLogger = GeneosLogger{os.Stderr, ERROR, true}

	Log   = log.New(Logger, "", 0)
	Debug = log.New(DebugLogger, "", 0)
	Error = log.New(ErrorLogger, "", 0)
)

type Level int

const (
	INFO Level = iota
	DEBUG
	WARNING
	ERROR
	FATAL
)

func (level Level) String() string {
	switch level {
	case INFO:
		return "INFO"
	case DEBUG:
		return "DEBUG"
	case ERROR:
		return "ERROR"
	case WARNING:
		return "WARNING"
	default:
		return "UNKNOWN"
	}
}

func init() {
	DisableDebugLog()
}

func EnableDebugLog() {
	Debug.SetOutput(DebugLogger)
}

func DisableDebugLog() {
	Debug.SetOutput(ioutil.Discard)
}

func (g GeneosLogger) Write(p []byte) (n int, err error) {
	var prefix string
	if g.ShowPrefix {
		prefix = fmt.Sprintf("%s %s: ", time.Now().Format(time.RFC3339), g.Level)
	}

	var line string
	switch g.Level {
	case FATAL:
		line = fmt.Sprintf("%s%s", prefix, p)
		io.WriteString(g.Writer, line)
		os.Exit(1)
	case ERROR:
		line = fmt.Sprintf("%s%s", prefix, p)
	case INFO:
		line = fmt.Sprintf("%s%s", prefix, p)

	case DEBUG:
		var fnName string = "UNKNOWN"
		pc, f, ln, ok := runtime.Caller(3)
		if ok {
			fn := runtime.FuncForPC(pc)
			if fn != nil {
				fnName = fn.Name()
			}
		}

		// filename is either relative (-trimpath) or the basename with a ./ prefix
		// this lets VSCode make the location clickable
		if filepath.IsAbs(f) {
			f = "./" + filepath.Base(f)
		}
		line = fmt.Sprintf("%s%s() %s:%d %s", prefix, fnName, f, ln, p)
	}
	return io.WriteString(g.Writer, line)
}
