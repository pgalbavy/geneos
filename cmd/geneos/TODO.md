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
* Docker Compose file build from selection of components
* check capabilities and not just setuid/root user
* Run REST commands against gateways
  * initially just a framework that picks up port number etc.
  * specific command output parsing
* command should show user information
* standalone collection agent
* centralised config
* web dashboard - mostly done, better port numbers and tls to do
* Support gateway2.gci format files
* Add a 'clone' command (rename without delete) - for backup gateways etc.
  * do both "mv" and "cp" working across remotes - tree walk needed
  * reset configs / clean etc.
* Redo template support, primarily for SANs but also gateways
  * document changes
* Stopping a remote (also for disable, delete, rename etc.) also means stopping all instances on it
* Update docs to include configuration file rebuilds, gateway includes etc.
* Look at 'sudo' support for remotes
* Review all log*.Fatal* calls
* web interface
  * first pass review configs
  * second to edit
  * use a REST interface
