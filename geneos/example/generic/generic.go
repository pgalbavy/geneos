package generic

import (
	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/samplers"
)

type GenericData struct {
	RowName string
	Column1 string
	Column2 string
}

type GenericSampler struct {
	samplers.Samplers
	localdata string
}

func New(s plugins.Connection, name string, group string) (*GenericSampler, error) {
	c := new(GenericSampler)
	c.Plugins = c
	c.SetName(name, group)
	return c, c.InitDataviews(s)
}

func (g *GenericSampler) InitSampler() error {
	example, err := g.Parameter("EXAMPLE")
	if err != nil {
		return nil
	}
	g.localdata = example

	columns, columnnames, sortcol, err := g.ColumnInfo(GenericData{})
	g.SetColumns(columns)
	g.SetColumnNames(columnnames)
	g.SetSortColumn(sortcol)
	return g.Headline("example", g.localdata)
}

func (p *GenericSampler) DoSample() error {
	var rowdata = []GenericData{
		{"row4", "data1", "data2"},
		{"row2", "data1", "data2"},
		{"row3", "data1", "data2"},
		{"row1", "data1", "data2"},
	}
	return p.UpdateTableFromSlice(rowdata)
}
