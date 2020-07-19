package memory

import (
	"fmt"
	"runtime"

	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/samplers"
)

type MemorySampler struct {
	samplers.Samplers
}

func New(s plugins.Connection, name string, group string) (m *MemorySampler, err error) {
	m = new(MemorySampler)
	m.Samplers.New(s, name, group)
	return
}

func (p *MemorySampler) InitSampler() (err error) {
	p.Headline("OS", runtime.GOOS)
	p.Headline("SampleInterval", fmt.Sprintf("%v", p.Interval()))
	return
}
