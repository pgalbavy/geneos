package xmlrpc // import "wonderland.org/geneos/pkg/xmlrpc"

import (
	"wonderland.org/geneos/pkg/logger"
)

func init() {
	// logger.EnableDebugLog()
}

var (
	Logger      = logger.Log
	DebugLogger = logger.LogDebug
	ErrorLogger = logger.LogError
)

// The XMLRPC type is a simple one
type XMLRPC interface {
	String() string
	IsValid() bool
}

func NewClient(url string, entityName string, samplerName string) (sampler Sampler, err error) {
	DebugLogger.Printf("%q, %q, %q", url, entityName, samplerName)
	c := Client{}
	c.SetURL(url)
	return c.NewSampler(entityName, samplerName)
}
