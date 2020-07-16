package memory

import (
	"fmt"
	"runtime"

	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/samplers"
)

type MemorySampler struct {
	*samplers.Samplers
}

func New(s plugins.Connection, name string, group string) (*MemorySampler, error) {
	m := &MemorySampler{&samplers.Samplers{}}
	m.Plugins = m
	m.SetName(name, group)
	return m, m.InitDataviews(s)
}

func (p *MemorySampler) InitSampler() (err error) {
	p.Dataview().Headline("OS", runtime.GOOS)
	p.Dataview().Headline("SampleInterval", fmt.Sprintf("%v", p.Interval()))
	return
}
