package memory

import (
	"fmt"
	"runtime"

	"wonderland.org/geneos"
	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/samplers"
)

func init() {
	// geneos.EnableDebugLog()
}

var (
	Logger      = geneos.Logger
	DebugLogger = geneos.DebugLogger
	ErrorLogger = geneos.ErrorLogger
)

type MemorySampler struct {
	samplers.Samplers
}

func New(p plugins.Connection, name string, group string) (*MemorySampler, error) {
	m := new(MemorySampler)
	m.Plugins = m
	return m, m.New(p, name, group)
}

func (p *MemorySampler) InitSampler() (err error) {
	p.Headline("OS", runtime.GOOS)
	p.Headline("SampleInterval", fmt.Sprintf("%v", p.Interval()))
	return
}
