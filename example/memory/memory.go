package memory

import (
	"fmt"
	"runtime"

	"wonderland.org/geneos/pkg/logger"
	"wonderland.org/geneos/pkg/plugins"
	"wonderland.org/geneos/pkg/samplers"
)

func init() {
	// logger.EnableDebugLog()
}

var (
	Logger      = logger.Logger
	DebugLogger = logger.DebugLogger
	ErrorLogger = logger.ErrorLogger
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
