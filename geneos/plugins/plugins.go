package plugins // import "wonderland.org/geneos/plugins"

import (
	"sync"
	"time"

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

// wrap calls to xmlrpc
func Sampler(url string, entityName string, samplerName string) (s Connection, err error) {
	sampler, err := xmlrpc.NewClient(url, entityName, samplerName)
	s = Connection{sampler}
	return
}

