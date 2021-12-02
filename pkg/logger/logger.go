package logger // import "wonderland.org/geneos/pkg/logger"

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
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
	ERROR
	WARNING
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
	case INFO:
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
	return g.Writer.Write([]byte(line))
}
