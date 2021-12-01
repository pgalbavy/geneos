package logger // import "wonderland.org/geneos/pkg/logger"

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"runtime" // placeholder
	"time"
)

type LogWriter struct {
	w          io.Writer
	ShowPrefix bool
}
type DebugLogWriter struct {
	w          io.Writer
	ShowPrefix bool
}
type ErrorLogWriter struct {
	w          io.Writer
	ShowPrefix bool
}

var (
	Logger      = LogWriter{log.Writer(), false}
	Log         = log.New(&Logger, "", 0)
	DebugLogger = DebugLogWriter{log.Writer(), true}
	LogDebug    = log.New(&DebugLogger, "", 0)
	ErrorLogger = ErrorLogWriter{log.Writer(), true}
	LogError    = log.New(&ErrorLogger, "", 0)
)

type Level int

const (
	Info Level = iota
	Debug
	Error
	Warning
)

func (level Level) String() string {
	switch level {
	case Info:
		return "INFO"
	case Debug:
		return "DEBUG"
	case Error:
		return "ERROR"
	case Warning:
		return "WARNING"
	default:
		return "UNKNOWN"
	}
}

func init() {
	DisableDebugLog()
}

func EnableDebugLog() {
	LogDebug.SetOutput(DebugLogger)
}

func DisableDebugLog() {
	LogDebug.SetOutput(ioutil.Discard)
}

func (f LogWriter) Write(p []byte) (n int, err error) {
	return writelog(Info, f.w, f.ShowPrefix, p)
}

func (f ErrorLogWriter) Write(p []byte) (n int, err error) {
	return writelog(Error, f.w, f.ShowPrefix, p)
}

func (f DebugLogWriter) Write(p []byte) (n int, err error) {
	return writelog(Debug, f.w, f.ShowPrefix, p)
}

func writelog(level Level, w io.Writer, printprefix bool, p []byte) (n int, err error) {
	var prefix string
	if printprefix {
		prefix = fmt.Sprintf("%s %s: ", time.Now().Format(time.RFC3339), level)
	}

	var line string
	switch level {
	case Info:
		line = fmt.Sprintf("%s%s", prefix, p)

	default:
		var fnName string = "UNKNOWN"
		pc, _, ln, ok := runtime.Caller(4)
		if ok {
			fn := runtime.FuncForPC(pc)
			if fn != nil {
				fnName = fn.Name()
			}
		}

		line = fmt.Sprintf("%s%s() line %d %s", prefix, fnName, ln, p)
	}
	return log.Writer().Write([]byte(line))
}
