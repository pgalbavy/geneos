package xmlrpc // import "wonderland.org/geneos/pkg/xmlrpc"

import (
	"wonderland.org/geneos"
)

func init() {
	// geneos.EnableDebugLog()
}

var (
	Logger      = geneos.Logger
	DebugLogger = geneos.DebugLogger
	ErrorLogger = geneos.ErrorLogger
)

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
