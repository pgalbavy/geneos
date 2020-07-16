# ITRS Geneos Golang packages


## Create a basic plugin

First, import the necessary packages

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

Now create the required methods. First a `New()` method that the main program will call to
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

You can also use the more compact form, replacing the verbose multiline initiaser
with a literal but this may become harder to read as the number of local data
items are added to the plugin struct:

```go
func New(s plugins.Connection, name string, group string) (*GenericSampler, error) {
	c := &GenericSampler{&samplers.Samplers{}, ""}
	c.Plugins = c

```

The next method is `InitSampler()` which is called once upon start-up of the sampler instance.
The first part of this example locates a parameter in the Geneos configurationa and assigns
is to the local data struct.

```go
func (g *GenericSampler) InitSampler() error {
	example, err := g.Dataview().Parameter("EXAMPLE")
	if err != nil {
		return nil
	}
    g.localdata = example

```

The second part is required to initialise the helper methods which we'll used see below:

```go

	columns, columnnames, sortcol, err := g.ColumnInfo(GenericData{})
	g.SetColumns(columns)
	g.SetColumnNames(columnnames)
	g.SetSortColumn(sortcol)
	return g.Dataview().Headline("example", g.localdata)
}
```

The final mandatory method is `DoSample()` which is called to update the data:

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

The call to `UpdateTableFromSlice()` uses the column data initialised earlier to
ensure the dataview is rendered correctly.

## More features

You can use tags to control the rendering of the data, like this example of a
CPU plugin for Windows:

```go
// +build windows
package cpu

import (
	"log"
	"time"

	"github.com/StackExchange/wmi"
	"wonderland.org/geneos/samplers"
)

// Win32_PerfRawData_PerfOS_Processor must be exported along with all it's
// fields so that methods in plugins package can output the results
type Win32_PerfRawData_PerfOS_Processor struct {
	Name                  string `column:"cpuName"`
	PercentUserTime       uint64 `column:"% User Time,format=%.2f %%"`
	PercentPrivilegedTime uint64 `column:"% Priv Time,format=%.2f %%"`
	PercentIdleTime       uint64 `column:"% Idle Time,format=%.2f %%"`
	PercentProcessorTime  uint64 `column:"% Proc Time,format=%.2f %%"`
	PercentInterruptTime  uint64 `column:"% Intr Time,format=%.2f %%"`
	PercentDPCTime        uint64 `column:"% DPC Time,format=%.2f %%"`
	Timestamp_PerfTime    uint64 `column:"OMIT"`
	Frequency_PerfTime    uint64 `column:"OMIT"`
}

// one entry for each CPU row in /proc/stats
type cpustat struct {
	cpus       map[string]Win32_PerfRawData_PerfOS_Processor
	lastsample float64
	frequency  float64
}

```

The tag is _column_ and the comma seperated tag values currently supported are:

* "name" - any value without an "=" is treated as a display name for the column
created from this field. The special name "OMIT" means that the fields should
not create a column, but the data will still be avilable for calculations etc.
* "format" - the _format_ tag is a Printf style fiormat string used to render the
value of the cell in the most appropriate way for the data
* "sort" - the _sort_ tag defines which one field - and only one field can be
selected - should be used to sort the resulting rows in the _Map_ rendering methods.
The valid values are an option leading + or - representing ascending or descending
order and the option suffix "num" to indicate a numeric sort. "sort=" means to
sort ascending in lexographical order, which is the same as "sort=+"

The _sort_ tag only applies to those dataviews populated from maps like this call
below:

```go
func (p *CPUSampler) DoSample() (err error) {
...
		err = p.UpdateTableFromMapDelta(stat.cpus, laststats.cpus, time.Duration(interval)*10*time.Millisecond)
```

The `UpdateTableFromSlice()` shown in the _generic_ example assumes that the slice has been
passed in the order required. Maps on the other hand have no defined order and the package
allows you to define the natural sort order. This can of course be overridden by the user
of the Geneos Active Console.
