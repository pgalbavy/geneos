module example

go 1.14

replace wonderland.org/geneos => ../

replace wonderland.org/geneos/pkg/logger => ../pkg/logger

replace wonderland.org/geneos/pkg/plugins => ../pkg/plugins

replace wonderland.org/geneos/pkg/samplers => ../pkg/samplers

replace wonderland.org/geneos/pkg/streams => ../pkg/streams

replace wonderland.org/geneos/pkg/xmlrpc => ../pkg/xmlrpc

retract (
    v0.3.0 // published too early
    v0.3.1 // published too early
    v0.3.2 // published too early
    v0.3.3 // published too early
    v0.3.4 // published too early
    v0.4.0 // published too early
    v0.4.1 // published too early
    v0.4.2 // published too early
)
