package logger // import "wonderland.org/geneos/pkg/logger"

import (
	"fmt"
	"io/ioutil"
	"log"
	"runtime" // placeholder
	"time"
)

var (
	Logger            = log.New(LogWriter{}, "", 0)
	DebugLogger       = log.New(DebugLogWriter{}, "", 0)
	ErrorLogger       = log.New(ErrorLogWriter{}, "", 0)
	zonename, zoneoff = time.Now().Zone()
)

type LogWriter struct{}
type DebugLogWriter struct{}
type ErrorLogWriter struct{}

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
	DebugLogger.SetOutput(DebugLogWriter{})
}

func DisableDebugLog() {
	DebugLogger.SetOutput(ioutil.Discard)
}

func (f LogWriter) Write(p []byte) (n int, err error) {
	return writelog(Info, p)
}

func (f ErrorLogWriter) Write(p []byte) (n int, err error) {
	return writelog(Error, p)
}

func (f DebugLogWriter) Write(p []byte) (n int, err error) {
	return writelog(Debug, p)
}

func writelog(level Level, p []byte) (n int, err error) {
	ts := time.Now().Format(time.RFC3339)

	var line string
	switch level {
	case Info:
		line = fmt.Sprintf("%s %s: %s", ts, level, p)

	default:
		var fnName string = "UNKNOWN"
		pc, _, ln, ok := runtime.Caller(4)
		if ok {
			fn := runtime.FuncForPC(pc)
			if fn != nil {
				fnName = fn.Name()
			}
		}

		line = fmt.Sprintf("%s %s: %s() line %d %s", ts, level, fnName, ln, p)
	}
	return log.Writer().Write([]byte(line))
}
