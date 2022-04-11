## v1.0.3-pre

* Added a -C option to 'init' to create default certs
* Added 'instance.setup.xml' include for all Gateways
  This is intended to provide global variables to a Gateway about the instance environment. It is rebuilt everything 'rebuild' is called, regardless of the ConfigRebuild setting, which is intended to control the main setup file. Further details in the template.
* You can now set Variables for San instances
  The form is `Variables=NAME=[TYPE:]VALUE,...` where TYPE defaults to `string`. As with other special setting types, using a hyphen before the NAME removes the setting. There is limited support for TYPEs, see the documentation.

## v1.0.2

* Change 'restart' to 'reload' in rebuild command
* Change Netprobe to San in 'init' Demo environment
* Add remote log tail support using a 1/2 second timer
* Import files into common directories (e.g. gateway_shared)
* Added mutex protected internal caching of components, remotes and ssh/sftp sessions
* Reworked move/copy to work with caches

## v1.0.1

* Fix order of directory creation in 'init'

## v1.0.0

* First Release