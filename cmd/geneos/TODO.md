# To Do list

(unordered)

* Positive confirmations of all commands unless quiet mode - PARTIAL
  * Should be an 'action taken' return from commands for output
  * create a seperate "verbose" logger and work through output to choose
  * or more if verbose ... logic
* Warnings when a name cannot be processed (but continue)
  * Help highlight typos rather than skip them
* Command line verbosity control - PARTIAL
* TLS support
  * output chain.pem file / or to stdout for sharing
* Docker / Compose file build from selection of components
* check capabilities and not just setuid/root user
* Run REST commands against gateways
  * initially just a framework that picks up port number etc.
  * specific command output parsing
* command should show user information
* SAN configuration support
  * config file. template, creation
  * command line options different
* standalone collection agent
* centralised config
* web dashboard - mostly done, better port numbers and tls to do
* FIX2 netprobe
* remote management support
  * add support for rwildcard remote instances, e.g. '@remote'
* Support gateway2.gci format files
* Add a 'clone' command (rename without delete) - for backup gateways etc.
  * do both "mv" and "cp" working across remotes - tree walk needed
  * reset configs / clean etc.
* Redo template support, primarily for SANs but also gateways
  * Have a templates/ directory under the top level component
  * The -t TEMPLATE option becomes a prefix for all matching files, e.g. -t netprobe or -t appname
  * -T to default ?
  * Output to single file, prefix as template name and set config to load this
  * init copies default templates to directrories, but only once - also pass init other templates instead?
  * Have a rebuild command/option in case templates change, also then support config settings to create templates
  * 'geneos config san X' - also does a reload


