package xmlrpc // import "wonderland.org/geneos/pkg/xmlrpc"

import (
	"wonderland.org/geneos/pkg/logger"
)

func init() {
	// logger.EnableDebugLog()
}

var (
	log      = logger.Log
	logDebug = logger.Debug
	logError = logger.Error
)

// The XMLRPC type is a simple one
type XMLRPC interface {
	String() string
	IsValid() bool
}

func NewClient(url string, entityName string, samplerName string) (sampler Sampler, err error) {
	logDebug.Printf("%q, %q, %q", url, entityName, samplerName)
	c := Client{}
	c.SetURL(url)
	return c.NewSampler(entityName, samplerName)
}
