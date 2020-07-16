package process

import (
	"fmt"
	"runtime"

	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/samplers"
)

type ProcessSampler struct {
	*samplers.Samplers
}

func New(s plugins.Connection, name string, group string) (*ProcessSampler, error) {
	c := &ProcessSampler{&samplers.Samplers{}}
	c.Plugins = c
	c.SetName(name, group)
	return c, c.InitDataviews(s)
}

func (p *ProcessSampler) InitSampler() (err error) {
	p.Dataview().Headline("OS", runtime.GOOS)
	p.Dataview().Headline("SampleInterval", fmt.Sprintf("%v", p.Interval()))

	columns, columnnames, sortcol, err := p.initColumns()
	if err == nil {
		p.SetColumns(columns)
		p.SetColumnNames(columnnames)
		p.SetSortColumn(sortcol)
	}
	return
}
