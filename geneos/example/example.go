package main

import (
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/streams"

	"example/cpu"
	"example/generic"
	"example/memory"
	"example/process"
)

func init() {
	// geneos.EnableDebugLog()
}

func main() {
	var wg sync.WaitGroup
	var interval time.Duration
	var (
		hostname                string
		port                    uint
		entityname, samplername string
	)

	flag.StringVar(&hostname, "h", "localhost", "Netprobe hostname")
	flag.UintVar(&port, "p", 7036, "Netprobe port number")
	flag.DurationVar(&interval, "t", 1*time.Second, "Globval DoSample Interval (min 1s)")
	flag.StringVar(&entityname, "e", "", "Default entity to connect")
	flag.StringVar(&samplername, "s", "", "Default sampler to connect")
	flag.Parse()

	if interval < 1*time.Second {
		log.Fatalf("supplied sample interval (%v) too short", interval)
	}

	// connect to netprobe
	url := fmt.Sprintf("http://%s:%v/xmlrpc", hostname, port)
	p, err := plugins.Sampler(url, entityname, samplername)
	if err != nil {
		log.Fatal(err)
	}

	m, err := memory.New(p, "memory", "SYSTEM")
	defer m.Close()
	m.SetInterval(interval)
	if err = m.Start(&wg); err != nil {
		log.Fatal(err)
	}

	c, err := cpu.New(p, "cpu", "SYSTEM")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	c.SetInterval(interval)
	if err = c.Start(&wg); err != nil {
		log.Fatal(err)
	}

	pr, err := process.New(p, "processes", "SYSTEM")
	defer pr.Close()
	pr.SetInterval(10 * time.Second)
	pr.Start(&wg)

	g, err := generic.New(p, "example", "SYSTEM")
	defer g.Close()
	g.SetInterval(interval)
	g.Start(&wg)

	powerwall, err := NewPW(p, "PW Meters", "Powerwall")
	defer powerwall.Close()
	powerwall.SetInterval(interval)
	powerwall.Start(&wg)

	streamssampler := "streams"
	sp, err := streams.Sampler(fmt.Sprintf("http://%s:%v/xmlrpc", hostname, port), entityname, streamssampler)
	if err != nil {
		log.Fatal(err)
	}
	wg.Add(1)
	go func() {
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for {
			<-tick.C
			err := sp.WriteMessage("teststream", time.Now().String()+" this is a test")
			if err != nil {
				log.Fatal(err)
				break
			}
		}
		wg.Done()
	}()

	wg.Wait()
}
