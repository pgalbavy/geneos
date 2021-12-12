# `geneos` management program

The `geneos` program will help you manage your Geneos environment, one server at a time. Some of it's features, existing and planned, include:

* Set-up a new environment with a series of simple commands
* Add new instances of common componments with sensible defaults
* Check the status of components
* Stop, start, restart components (without unnecessary delays)
* Support fo existing gatewayctl/netprobectl/licdctl scripts and their file and configuration layout
* Convert existing set-ups to JSON based configs with more options
* Edit individual settings of instances
* Automatically download and install the latest packages (authentication required, for now)
* Update running versions - stop, update, start


## Getting Started

You can either download a sinfle binary - or build and install from source, if you have Go 1.17+ installed:

`go install wonderland.org/geneos/cmd/geneos@latest`

or - if you want the bleeding edge -

`go install wonderland.org/geneos/cmd/geneos@HEAD`

Make sure that the `geneos` program is in your normal `PATH` - or that $HOME/go/bin is if you used the method above - to make things simpler.

### Existing Installation

If you have an existing Geneos installation that you manage with the legacy `gatewayctl` / `netprobectl` commands then you can try these:

`geneos list` - show all components
`geneos status` - show their running status
`geneos show` - show the default configuration values

None of these commands should have any side-effects but others will not only start or stop processes but may also convert configuration files to JSON format without further prompting. Old `.rc` files are backed-up with a `.rc.orig` extension.

Note: You will have to set an environment variable `ITRS_HOME` pointing to the top-level directory of an existing installation or use:

`geneos set ITRSHome=/path/to/install`

This path is where the `packages` and `gateway` directories live. If you do not have an existing installation then wait until we initialise one below.

## New Installation

```bash
geneos init
geneos set DownloadUser="email" DownloadPass="password"
geneos install latest
geneos create gateway Gateway1
geneos create netprobe Netprobe1
geneos create licd Licd1
geneos upload licd Licd1 geneos.lic
geneos start
```

You still have to configure the Gateway to connect to the Netprobe, but all three components should now be running. You can check with:

`geneos status`

## Usage

Please note that the precise combination of commands and parameters is changing all the time. This list below is mostly, but not completely, up-to-date.

The general syntax is:

`geneos COMMAND [optional component type] [optional names...]`

There are a number of special cases, these are detailed below.

### Component Types

The following component types (and their aliases) are supported:

* `any` or empty
* `gateway` or `gateways`
* `netprobe`, `netprobes`, `probe` or `probes`
* `licd` or `licds`
* `webserver`, `webservers`, `webdashboard`. `dashboards`

These names are also reserved words and you cannot configure or manage components with those names. This means that you cannot have a gateway called `gateway` or a probe called `netprobe`. If you do already have instances with these names then you will have to be careful migrating. See more below.

### Commands

#### Security and Running as Root

The program has been written in such a way that is should be safe to install SETUID root or run using `sudo` for almost all cases. The program will refuse to accidentally run an instance as root unless the `User` config parameter is explicitly set - for example when a Netprobe needs to run as root. As with many complex programs, care should be taken and privileged execution should be used when required.

#### Global Commands

* `geneos version`
Show the current version of the `geneos` program, which should match the tag of the overall `geneos` package.

* `geneos help`
General help, initially a list of all the supported commands.

* `geneos list [component]`
Output a list of all configured components. If a compoent type is supplied then list just that particular type.

* `geneos status [component] [names...]`
As above but show the running status of each matching component. If no names are given than all components are shown.

#### Environment Commands

* `geneos init [username] [directory]`
Initialise the environment. 

* `geneos show [global|user]`
Show the running configuration or, if `global` or `user` is supplied then the respective on-disk configuration files. Passwords are simplisticly redated.
The instance specific `show` command is described below.

* `geneos set [global|user] KEY=VALUE...`
Set a program-wide configuration option. The default is to update the `user` configuration file. If `global` is given then the user has to have appropriate privileges to write to the global config file (`/etc/geneos/geneos.json`). Multiple KEY=VALUE pairs can be given but only fields that are recognised are updated.
The instance specific version of the `set` command is described below.
#### Package Managemwent Commands

* `geneos install [archives...]` - Not yet
Install a release archive in the `packages` hierarchy.

* `geneos update component`
Update the component base binary link

#### Instance Control Commands

* `geneos start [component] [name...]`
Start a Geneos component. If no name is supplied or the special name `all` is given then all the matching Geneos components are started.

* `geneos stop [component] [name...]`
Like above, but stops the component(s)

* `geneos kill [component] [name...]`
Stops components immediately (SIGKILL is sent)

* `geneos restart [component] [name...]`
Restarts matching geneos components. Each component is stopped and started in sequence. If all components should be down before starting up again then use a combination of `start` and `stop` from above.

* `geneos reload|refresh [component] name [name...]`
Cause the component to reload it's configuration or restart as appropriate.

* `geneos disable [component] [name...]`
Stop and disable the selected compoents by placing a file in wach working directory with a `.disable` extention

* `geneos enable [component] [name...]`
Remove the `.disable` lock file and start the selected components

* `geneos clean [component] [names]`
Clean up component

#### Instance Configuration Commands

* `geneos create [component] name [name...]`
Create a Geneos component configuration.

* `geneos migrate [component] [instance...]`
Migrate legacy `.rc` files to `.json` and backup the original file with an `.orig` extension. This backup file can be used by the `revert` command, below, to restore the original `.rc` file(s)

* `geneos revert [component] [instance...]`
Revert to the original configuration files, deleting the `.json` files. Note that the `.rc` files are never changed and any configuration changes to the `.json` configuration will not be retained.

* `geneos command [component] [name...]`
Shows details of the full command used for the component and any extra environment variables found in the configuration.

* `geneos rename [component] name newname`
Rename the compoent, but this only affects the container directory, this programs JSON configursation file and does not update the contents of any other files.

* `geneos delete component name`
Deletes the disabled component given. Only works on components that have been disabled beforehand.

* `geneos edit [user|component] [names]`
Open an editor for the selected instances or user JSON config file. Will accept wild or multiple instance names.

* `geneos upload [component] name [file|url|-]`
Upload a file into an instance working directory, from local file, url or stdin and backup previous file. The file can also specify the destination name and sub-directory, which will be created if it does not exist. Examples of valid files are:
  `geneos upload gateway Example gateway.setup.xml`
  `geneos upload gateway Example https://server/files/gateway.setup.xml`
  `geneos upload gateway Example gateway.setup.xml=mygateway.setup.xml`
  `geneos upload gateway Example scripts/newscript.sh=myscript.sh`
  `geneos upload gateway Example scripts/=myscript.sh`
  `cat someoutput | geneos upload gateway Example config.json=-`
Like other commands that write to the file system is can safely be run as root as the destination directory and file will be changed to be owned by either the instance or the default user, with the caveat that any intermediate directrories above the destination sub-directory (e.g. the first two in `my/long/path`) will be owned by root.
`geneos clean` will remove backup files. Principal use for license token files, XML configs, scripts.

## Configuration Files

### General Configuration

* `/etc/geneos/geneos.json` - Global options
* `${HOME}/.config/geneos.json` - User options

General options are loaded from the global config file first, then the user level one. The current options are:

* `ITRSHome`
The home directory for all other commands. See [Directory Layout](#directory-layout) below. If set the environment variable ITRS_HOME overrides any settings in the files. This is to maintain backward compatibility with older tools. The default, if not set anywhere else, is the home directory of the user running the command or, if running as root, the home directory of the `geneos` or `itrs` users (in that order). (To be fully implemented)

* `DownloadURL`
The base URL for downloads for automating installations. Not yet used.
If files are locally downloaded then this can either be a `file://` style URL or a directory path.

* `DownloadUser` & `DownloadPass`

* `DefaultUser`
Principally used when running with elevated priviliedge (setuid or `sudo`) and a suitable username is not defined in instance configurations or for file ownership of shared directories.

* `GatewayPortRange` & `NetprobePortRange` & `LicdPortRange`


### Component Configuration

For compatiblity with earlier tools, the per-component configurations are loaded from `.rc` files in the working directory of each component. The configuration names are also based on the original names, hence they can be obscure. the `migrate` command allows for the conversion of the `.rc` file to a JSON format one, the original `.rc` file being renamed to end `.rc.orig` and allowing the `revert` command to restore the original (without subsequent changes).

If you want to change settings you should first `migrate` the configuration and then use `set` to make changes.

Note that execution mode (e.g. `GateMode`) is not supported and all components run in the background.

## Directory Layout

The ITRSHome setting or the environment variable `ITRS_HOME` points to the base directory for all subsequent operations. The basic layout follows that of the original `gatewayctl` etc. including:

```
packages/
  gateway/
    [versions]/
    active_prod -> [chosen version]
  netprobe/
  licd/
gateway/
netprobe/
licd/
```

The `bin/` directory and the default `.rc` files are **ignored** so be aware if you have customised anything in `bin/`.

As a very quick recap, each component directory will have a subdirectory with the plural of the name (`gateway` -> `gateways`) which will contain multiple subdirectories, one per instance, and these act as the configuration and working directories for the individual processes. Taking an example gateway called `Gateway1` the path will be:

`${ITRS_HOME}/gateway/gateways/Gateway1`

This directory will be the working directory of the process and also contain an `.rc` configuration file as well as a `.txt` file to capture the `STDOUT` and `STDERR` of the process, like this:

```
gateway.rc
gateway.txt
```

There will also be an XML setup file and so on.
 