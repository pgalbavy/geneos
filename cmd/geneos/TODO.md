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
  * output chain.pem file on stdout for sharing
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
* modularise component types, make future addins easier - PARTIAL
* remote management support
  * transparency - all except `logs -f` should work with ssh and sftp
  * add support for rwildcard remote instances, e.g. '@remote'
* Rename 'upload' to 'copy' or something more intuitive
* Examine integrating `upload` into `add` - see above too
* Support gateway2.gci format files
* Add a 'clone' command (rename without delete)



