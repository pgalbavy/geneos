package cpu

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

type CPUSampler struct {
	samplers.Samplers
	cpustats cpustat
}

func New(s plugins.Connection, name string, group string) (*CPUSampler, error) {
	DebugLogger.Print("called")
	c := new(CPUSampler)
	c.Plugins = c
	return c, c.New(s, name, group)
}

func (p *CPUSampler) InitSampler() (err error) {
	DebugLogger.Print("called")
	p.Headline("OS", runtime.GOOS)
	p.Headline("SampleInterval", fmt.Sprintf("%v", p.Interval()))

	// call internal OS column init
	columns, columnnames, sortcol, err := p.initColumns()
	p.SetColumns(columns)
	p.SetColumnNames(columnnames)
	p.SetSortColumn(sortcol)
	return
}
