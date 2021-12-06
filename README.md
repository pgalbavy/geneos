# ITRS Geneos Go Tools

This is a collection of packages and tools written in Go

## `geneos` - Control your Geneos environment

The `cmd/geneos` directory contains a standalone program to manage your local Geneos components. It is written to be compatible with gatewayctl/netprobectl/etc. configuration layouts and files but with a more unified approach and all options only available through command line parameters, not prompts, allowing more automation and less hands-on required.

## `libemail.so` drop in replacement

The `tools/libemailgo` directory contains a drop-in replacement for the standard Geneos libemail.so SMTP mailer (the `SendMail` function) while providing more up-to-date SMTP support with TLS and authentication support plus an additional `GoSendMail` entry point that uses Go text and HTML templates instead of the standard text formats allowing richer alerting email.

Requires Go 1.17+

## XML-RPC Go bindings

The `pkg` directory contains set of low-level mappings in Go to provide a one-to-one interface to the XML-RPC functions to the `api` and `api-streams` plugins in the Netprobe and also a number of higher level functions that wrap these in slightly more - but not fully - idiomatic Go. 

There are some basic examples of use in the `example` directory.