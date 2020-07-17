package xmlrpc // import "wonderland.org/geneos/xmlrpc"

import (
	"wonderland.org/geneos"
)

var (
	Logger      = geneos.Logger
	DebugLogger = geneos.DebugLogger
	ErrorLogger = geneos.ErrorLogger
)

type XMLRPC interface {
	ToString() string
	IsValid() bool
}

func NewClient(url string, entityName string, samplerName string) (sampler Sampler, err error) {
	Logger.Printf("%q, %q, %q", url, entityName, samplerName)
	c := Client{}
	c.SetURL(url)
	return c.NewSampler(entityName, samplerName)
}
