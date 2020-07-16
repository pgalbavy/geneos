package streams

import (
	"wonderland.org/geneos/xmlrpc"
)

type Stream struct {
	xmlrpc.Sampler
}

// Sampler - wrap calls to xmlrpc
func Sampler(url string, entityName string, samplerName string) (s Stream, err error) {
	sampler, err := xmlrpc.NewClient(url, entityName, samplerName)
	s = Stream{sampler}
	return
}
