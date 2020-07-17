# ITRS Geneos Golang packages

This set of packages provides a wrapper around low-level XML-RPC calls to send data to a Geneos Netprobe.

The documentation for the underlying API is here:

https://docs.itrsgroup.com/docs/geneos/current/Netprobe/api/xml-rpc-api.html

While direct mappings from golang to the API are available in the _xmlrpc_ package most users will want to look at the higher-level _samplers_ and _streams_ packages that try to implement easier to use high-level tool.

## Examples of use

The _examples_ package directory contains a number of simple implmentations of common plugin types that show how to use the different types of data update methods.

The _examples/generic_ directory is described in further detail below. It uses this method to deliver updates:

```go
func (s Samplers) UpdateTableFromSlice(rowdata interface{}) error
```

The other two methods both take maps as follows:

```go
func (s *Samplers) UpdateTableFromMap(data interface{}) error
```
```go
func (s *Samplers) UpdateTableFromMapDelta(newdata, olddata interface{}, interval time.Duration) error
```

The `UpdateTableFromMapDelta()` also takes an `time.Duration` interval that allows scaling of the difference between the two datasets. 



## Create a basic plugin

First, import the necessary packages

```go
package generic

import (
	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/sampler"
)
```

Next, create two structs, one to hold the per-sample data and another to hold the Sampler amd any other local data that is needed for the lifetime of the sampler:

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

Now create the required mandatory methods. There are three and they must meet this interface (from the samplers package):

```go
type SamplerInstance interface {
	New(plugins.Connection, string, string) *SamplerInstance
	InitSampler(*SamplerInstance) (err error)
	DoSample(*SamplerInstance) (err error)
}
```


First a `New()` method that the main program will call to create an instance of the plugin - the sampler - does some housek:

```go
func New(s plugins.Connection, name string, group string) (*GenericSampler, error) {
	c := new(GenericSampler)
	c.Samplers = new(samplers.Samplers)
	c.Plugins = c
	c.SetName(name, group)
	return c, c.InitDataviews(s)
}
```

You can also use the more compact form, replacing the verbose multiline initiaser with a literal but this may become harder to read as the number of local data
items are added to the plugin struct:

```go
func New(s plugins.Connection, name string, group string) (*GenericSampler, error) {
	c := &GenericSampler{&samplers.Samplers{}, ""}
	c.Plugins = c

```

The next method is `InitSampler()` which is called once upon start-up of the sampler instance. The first part of this example locates a parameter in the Geneos configurationa and assigns is to the local data struct.

```go
func (g *GenericSampler) InitSampler() error {
	example, err := g.Dataview().Parameter("EXAMPLE")
	if err != nil {
		return nil
	}
	g.localdata = example

```

It is worth noting at this point that the `InitSampler()` being called only once means that if there is any change in the Geneos configuration there is no way for the running program to notice. The XML-RPC API is stateless (we'll ignore the heartbeat functions for now) and these plugins may not notice a Netprobe or related restart. So, the `Parameter()` call above is only an example and should probably be refreshed using a timer, but not every sample most likely.   

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

The call to `UpdateTableFromSlice()` uses the column data initialised earlier to ensure the dataview is rendered correctly.

## More features

You can use tags to control the rendering of the data, like this example of a CPU plugin for Windows:

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

* "name" - any value without an "=" is treated as a display name for the column created from this field. The special name "OMIT" means that the fields should not create a column, but the data will still be avilable for calculations etc.
* "format" - the _format_ tag is a `Printf` style format string used to render the value of the cell in the most appropriate way for the data
* "sort" - the _sort_ tag defines which one field - and only one field can be selected - should be used to sort the resulting rows in the _Map_ rendering methods. The valid values are an option leading + or - representing ascending or descending order and the option suffix "num" to indicate a numeric sort. "sort=" means to sort ascending in lexographical order, which is the same as "sort=+"

The _sort_ tag only applies to those dataviews populated from maps like this call below:

```go
func (p *CPUSampler) DoSample() (err error) {
...
		err = p.UpdateTableFromMapDelta(stat.cpus, laststats.cpus, time.Duration(interval)*10*time.Millisecond)
```

The `UpdateTableFromSlice()` shown in the _generic_ example assumes that the slice has been passed in the order required. Maps on the other hand have no defined order and the package allows you to define the natural sort order. This can of course be overridden by the user of the Geneos Active Console.

## Initialise and start-up

To use your new plugin in a program, use it like this:

```go
package main

import (
...
	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/streams"

	"example/generic"
)
```

Do normal start-up configuration, process command line args etc. and then initialise the
`Sampler` connection like this: 

```go
func main() {
...

	// connect to netprobe
	url := fmt.Sprintf("http://%s:%v/xmlrpc", hostname, port)
	s, err := plugins.Sampler(url, entityname, samplername)
	if err != nil {
		log.Fatal(err)
	}
```

Once you have your _sampler_ connection call the `New()` method with _dataview_ and _group_ names. The _group_ can be an empty string. Set the _interval_ as a Go `time.Duration` value. The default (and minimum) is one second. Finally `Start()` the sampler by passing a `sync.WaitGroup` that you can later `Wait()` on so the program doesn't exit while the sampler runs.

```go
	g, err := generic.New(s, "example", "SYSTEM")
	defer g.Close()
	g.SetInterval(interval)
	g.Start(&wg)

	wg.Wait()
}

```


## Logging

There is a basic logging interface to allow for common logging formats for any plugins and other data. To use this you have to import the top-level _geneos_ package and then, for simplicity, make local copies of the Loggers, like this:

```go
import (
...
	"wonderland.org/geneos"
)

var (
	Logger      = geneos.Logger
	DebugLogger = geneos.DebugLogger
	ErrorLogger = geneos.ErrorLogger
)
```

Then all of the normal logging methods will work.

The `DebugLogger` is turned off by default and can be enabled using `geneos.EnableDebugLog()` and then disabled again using `geneos.DisableDebugLog()`. It should be noted that these calls will enable and disable debug logging globally for the whole process including other running samplers.

## Streams

Basic support for _streams_ are included. Streams must be predefined in the Geneos configuration and sending messages to a non-existent stream name results in an error.

```go
import (
	"wonderland.org/geneos/streams"
)
```

```go
func main() {
...
	streamsampler := "streams"
	sp, err := streams.Sampler(fmt.Sprintf("http://%s:%v/xmlrpc", hostname, port), entityname, streamsampler)
	if err != nil {
		log.Fatal(err)
	}

	err := sp.WriteMessage("teststream", time.Now().String()+" this is a test")
	if err != nil {
		log.Fatal(err)
		break
	}
```

Note that the sampler name is always different to the normal dataview destination as the plugin on the Geneos side must be an _api-streams_ one. Also there is no `Close()` method. At the moment there is no direct support for heartbeats.
