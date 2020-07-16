package main

// example of how you would build a sampler into the main package
//
// this example does NOT use the helper functions for dealing with structures,
// but it could

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/samplers"
)

type PowerwallSampler struct {
	*samplers.Samplers
	pwurl string
}

func NewPW(s plugins.Connection, name string, group string) (*PowerwallSampler, error) {
	c := &PowerwallSampler{&samplers.Samplers{}, ""}
	c.Plugins = c
	c.SetName(name, group)
	return c, c.InitDataviews(s)
}

var pwcols = []string{
	"meterName",
	"last_communication_time",
	"instant_power",
	"instant_reactive_power",
	"instant_apparent_power",
	"frequency",
	"energy_exported",
	"energy_imported",
	"instant_average_voltage",
	"instant_total_current",
	"i_a_current",
	"i_b_current",
	"i_c_current",
	"timeout",
}

var pwrows = []string{
	"site",
	"battery",
	"load",
	"solar",
}

func (p *PowerwallSampler) InitSampler() (err error) {
	d := p.Dataview()
	pwurl, err := d.Parameter("POWERWALL_URL")
	if err != nil {
		return
	}
	d.Headline("powerwallURL", p.pwurl)
	p.pwurl = pwurl + "/meters/aggregates"
	return
}

func (p PowerwallSampler) DoSample() (err error) {
	d := p.Dataview()
	if p.pwurl == "" {
		err = fmt.Errorf("No URL defined in sampler parameters (POWERWALL_URL)")
		return
	}
	resp, err := http.Get(p.pwurl)
	if err != nil {
		log.Fatal(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	var data map[string]map[string]interface{}
	err = json.Unmarshal(body, &data)
	// all numbers are float64 at this point, don't forget to reconvert

	table := make([][]string, len(pwrows))

	for row, rowname := range pwrows {
		table[row] = make([]string, len(pwcols)+1)
		table[row][0] = rowname
		for col, colname := range pwcols[1:] {
			dv := data[rowname][colname]
			if dv == nil {
				continue
			}
			var tv string
			switch dv.(type) {
			case string:
				tv = dv.(string)
			case float64:
				tv = fmt.Sprintf("%.2f", dv.(float64))
			default:
				tv = fmt.Sprintf("%v", dv)
			}
			table[row][col+1] = tv
		}
	}
	err = d.UpdateTable(pwcols, table...)
	if err != nil {
		log.Fatal(err)
	}
	return
}
