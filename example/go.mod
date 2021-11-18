module example

go 1.14

replace wonderland.org/geneos => ../

replace wonderland.org/geneos/pkg/logger => ../pkg/logger

replace wonderland.org/geneos/pkg/plugins => ../pkg/plugins

replace wonderland.org/geneos/pkg/samplers => ../pkg/samplers

replace wonderland.org/geneos/pkg/streams => ../pkg/streams

replace wonderland.org/geneos/pkg/xmlrpc => ../pkg/xmlrpc

require (
	github.com/StackExchange/wmi v1.2.1
	github.com/mackerelio/go-osstat v0.2.1
	wonderland.org/geneos/pkg/logger v0.0.0-00010101000000-000000000000
	wonderland.org/geneos/pkg/plugins v0.0.0-00010101000000-000000000000
	wonderland.org/geneos/pkg/samplers v0.0.0-00010101000000-000000000000
	wonderland.org/geneos/pkg/streams v0.0.0-00010101000000-000000000000
	wonderland.org/geneos/pkg/xmlrpc v0.0.0-00010101000000-000000000000 // indirect
)
