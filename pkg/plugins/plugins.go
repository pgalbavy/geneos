package plugins // import "wonderland.org/geneos/plugins"

import (
	"sync"
	"time"

	"wonderland.org/geneos"
	"wonderland.org/geneos/xmlrpc"
)

// all Plugins must implement these methods
type Plugins interface {
	SetInterval(time.Duration)
	Interval() time.Duration

	Start(*sync.WaitGroup) error
	Close() error
}

type Connection struct {
	xmlrpc.Sampler
}

var (
	Logger      = geneos.Logger
	DebugLogger = geneos.DebugLogger
	ErrorLogger = geneos.ErrorLogger
)

// wrap calls to xmlrpc
func Sampler(url string, entityName string, samplerName string) (s Connection, err error) {
	DebugLogger.Printf("called")
	sampler, err := xmlrpc.NewClient(url, entityName, samplerName)
	s = Connection{sampler}
	return
}
