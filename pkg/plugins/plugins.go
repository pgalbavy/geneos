package plugins // import "wonderland.org/geneos/pkg/plugins"

import (
	"sync"
	"time"

	"wonderland.org/geneos/pkg/logger"
	"wonderland.org/geneos/pkg/xmlrpc"
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
	log      = logger.Log
	logDebug = logger.Debug
	logError = logger.Error
)

// wrap calls to xmlrpc
func Sampler(url string, entityName string, samplerName string) (s Connection, err error) {
	logDebug.Printf("called")
	sampler, err := xmlrpc.NewClient(url, entityName, samplerName)
	s = Connection{sampler}
	return
}
