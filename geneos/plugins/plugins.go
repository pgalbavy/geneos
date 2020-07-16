package plugins // import "wonderland.org/geneos/plugins"

import (
	"sync"
	"time"

	"wonderland.org/geneos"
	"wonderland.org/geneos/xmlrpc"
)

// all Plugins must implement these methods
type Plugins interface {
	SetName(string, string)
	Name() (string, string)

	SetInterval(time.Duration)
	Interval() time.Duration

	Dataview() *xmlrpc.Dataview
	InitDataviews(Connection) error

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
	DebugLogger.Printf("testing")

	sampler, err := xmlrpc.NewClient(url, entityName, samplerName)
	s = Connection{sampler}
	return
}
