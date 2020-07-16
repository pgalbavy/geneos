package xmlrpc // import "wonderland.org/geneos/xmlrpc"

import (
	"log"
	"os"
)

type XMLRPC interface {
	ToString() string
	IsValid() bool
}

var (
	Logger      *log.Logger
	DebugLogger *log.Logger
	ErrorLogger *log.Logger
)

func init() {
	Logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile|log.Lmsgprefix)
	DebugLogger = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Llongfile|log.Lmsgprefix)
	ErrorLogger = log.New(os.Stdout, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile|log.Lmsgprefix)
}

func NewClient(url string, entityName string, samplerName string) (sampler Sampler, err error) {
	DebugLogger.Printf("Connect(): %q, %q, %q", url, entityName, samplerName)
	c := Client{}
	c.SetURL(url)
	return c.NewSampler(entityName, samplerName)
}
