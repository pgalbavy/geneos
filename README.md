# ITRS Geneos Golang packages


## Create a basic plugin

First, import the basic Geneos packages

```go
package generic

import (
    "wonderland.org/geneos/plugins"
    "wonderland.org/geneos/sampler"
)
```

Next, create two structs, one to hold the per-sample data and another to hold the Sampler
amd any other local data that is needed for the lifetime of the sampler:

```go
type GenericData struct {
	RowName string
	Column1 string
	Column2 string
}

type GenericSampler struct {
	*samplers.Samplers
	localdata string
}
```

Now create the required methods. First a New() method that the main program will call to
create an instance of the plugin - the sampler - does some housek:

```go
func New(s plugins.Connection, name string, group string) (*GenericSampler, error) {
	c := new(GenericSampler)
	c.Samplers = new(samplers.Samplers)
	c.Plugins = c
	c.SetName(name, group)
	return c, c.InitDataviews(s)
}
```

You can also use the more compact form:

```go
func New(s plugins.Connection, name string, group string) (*GenericSampler, error) {
	c := &GenericSampler{&samplers.Samplers{}, ""}
	c.Plugins = c
	c.SetName(name, group)
	return c, c.InitDataviews(s)
}
```

The next method is InitSampler() which is called once upon start-up of the sampler instance.
The first part of this example locates a parameter in the Geneos configurationa and assigns
is to the local data struct.

```go
func (g *GenericSampler) InitSampler() error {
	example, err := g.Dataview().Parameter("EXAMPLE")
	if err != nil {
		return nil
	}
    g.localdata = example
    ...
```

The second part is required to initialise the helper methods which we'll used see below:

```go
    ...
	columns, columnnames, sortcol, err := g.ColumnInfo(GenericData{})
	g.SetColumns(columns)
	g.SetColumnNames(columnnames)
	g.SetSortColumn(sortcol)
	return g.Dataview().Headline("example", g.localdata)
}
```

The final mandatory method is DoSample() which is called to update the data:

```go
func (p *GenericSampler) DoSample() error {
	var rowdata = []GenericData{
		{"row4", "data1", "data2"},
		{"row2", "data1", "data2"},
		{"row3", "data1", "data2"},
		{"row1", "data1", "data2"},
	}
	return p.UpdateTableFromSlice(rowdata)
}
```

The call to UpdateTableFromSlice() uses the column data initialised earlier to
ensure the dataview is rendered correctly.
