package geneos // import "wonderland.org/geneos"

import (
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
	return writelog("INFO", p)
}

func (f ErrorLogWriter) Write(p []byte) (n int, err error) {
	return writelog("ERROR", p)
}

func (f DebugLogWriter) Write(p []byte) (n int, err error) {
	return writelog("DEBUG", p)
}

func writelog(level string, p []byte) (n int, err error) {
	var fnName string = "UNKNOWN"
	pc, _, line, ok := runtime.Caller(4)
	if ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			fnName = fn.Name()
		}
	}

	log.Printf("%s %s: %s() line %d %s", zonename, level, fnName, line, p)
	return len(p), nil
}
