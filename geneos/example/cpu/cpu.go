package cpu

import (
	"fmt"
	"runtime"

	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/samplers"
)

type CPUSampler struct {
	*samplers.Samplers
	cpustats cpustat
}

func New(s plugins.Connection, name string, group string) (*CPUSampler, error) {
	c := &CPUSampler{&samplers.Samplers{}, cpustat{}}
	c.Plugins = c
	c.SetName(name, group)
	return c, c.InitDataviews(s)
}

func (p *CPUSampler) InitSampler() (err error) {
	p.Dataview().Headline("OS", runtime.GOOS)
	p.Dataview().Headline("SampleInterval", fmt.Sprintf("%v", p.Interval()))

	// call internal OS column init
	columns, columnnames, sortcol, err := p.initColumns()
	p.SetColumns(columns)
	p.SetColumnNames(columnnames)
	p.SetSortColumn(sortcol)
	return
}
