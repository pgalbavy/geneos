module example

go 1.17

require (
	github.com/StackExchange/wmi v1.2.1
	github.com/mackerelio/go-osstat v0.2.1
	wonderland.org/geneos v0.5.3
)

require (
	github.com/go-ole/go-ole v1.2.5 // indirect
	golang.org/x/sys v0.0.0-20210113181707-4bcb84eeeb78 // indirect
)

replace wonderland.org/geneos => ../

retract (
	v0.4.2 // published too early
	v0.4.1 // published too early
	v0.4.0 // published too early
	v0.3.4 // published too early
	v0.3.3 // published too early
	v0.3.2 // published too early
	v0.3.1 // published too early
	v0.3.0 // published too early
)
