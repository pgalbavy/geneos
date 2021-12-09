# `geneos`

The `geneos` program will help you manage your Geneos components. Some of it's features, existing and planned, include:

* Set-up a new environment with a single command
* Add new instances of common componments with sensible defaults
* Check the status of components
* Stop, start, restart components (without those unnecessary delays from `sleep` in traditional shell scripts)
* Support fo existing gatewayctl/netprobectl/licdctl scripts and their file and configuration layout
* Convert existing set-ups to JSON based configs with more options

* 

## Installation

Either download the standalone binary or build from source:

```
git clone https://github.com/pgalbavy/geneos.git
cd geneos/cmd/geneos
go build
- or -
go install
```

Make sure that the `geneos` program is in your normal `PATH` to make execution easier. You should be able to immediately run these:

`geneos list` - show all components
`geneos status` - show their running status
`geneos show` - show the default configuration values

Neither of these commands should have any side-effects but others will not only start or stop processes but may also convert configuration files to JSON format without further prompting. Old `.rc` files are backed-up with a `.rc.orig` extension.

## Usage

Please note that the precise combination of commands and parameters is changing with every commit. This list below is mostly, but not completely, up-to-date.

### Components

The following component types (and their aliases) are supported:

* `any` or empty
* `gateway` or `gateways`
* `netprobe`, `netprobes`, `probe` or `probes`
* `licd` or `licds`
* `webserver`, `webservers`, `webdashboard`. `dashboards`

The above names are also treated as reserved words and you cannot configure or manage components with those names. This means that you cannot have a gateway called `gateway` or a probe called `netprobe`. This may cause some issues migrating existing installations. See below for more. 

The following commands are supported. Each command may treat arguments differently.

* `geneos list [component]`
Output a list of all configured components. If a compoent is supplied then restrict the list to that particular type.

* `geneos status [component] [names...]`
As above but show the running status of each matching component. If no names are given than all components are shown.

* `geneos version`
Show the current version of the `geneos` program, which should match the tag of the overall `geneos` package.

* `geneos help`
General help, initially a list of all the supported commands.

* `geneos init`
Initialise the environment.

* `geneos show`
* `geneos set`
* `geneos migrate`
* `geneos revert`
  View or change settings. The two forms of the command, `geneos config [show|set] global` and `geneos config [show|set] user` act on non-component specific settings such as the root directory and the base URL for downloading release archives. Then component specific versions also support `migrate` and `revert` to transform existnig `.rc` files to `.json` and to revert the changes. A more complete description is below.

  The optional environment variables in component configurations cannot yet be maintained with the command line tools.

* `geneos command [component] [name...]`
Shows details of the command syntax used for the component and any extra environment variables found in the configuration.
(This may be merged into the `show` command above)

* `geneos create [component] name [name...]`
Create a Geneos component configuration.

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

* `geneos install [archives...]` - Not yet
Install a release archive in the `packages` hierarchy.

* `geneos update component`
Update the component base binary link

* `geneos disable [component] [name...]`
Stop and disable the selected compoents by placing a file in wach working directory with a `.disable` extention

* `geneos enable [component] [name...]`
Remove the `.disable` lock file and start the selected components

* `geneos rename [component] name newname`
Rename the compoent, but this only affects the container directory, this programs JSON configursation file and does not update the contents of any other files.

* `geneos delete component name`
Deletes the disabled component given. Only works on components that have been disabled beforehand.

* `geneos edit [user|component] [names]`
Open an editor for the selected instances or user config file

* `geneos clean [component] [names]`
Clean up component

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

The `bin/` directory and the default `.rc` files are **ignored** so be aware if you have changed anything in `bin/` it will not be used.

As a very quick recap, each component directory will have a subdirectory with the plural of the name (`gateway` -> `gateways`) which will contain multiple subdirectories, one per instance, and these act as the configuration and working directories for the individual processes. Taking an example gateway called `Gateway1` the path will be:

`${ITRS_HOME}/gateway/gateways/Gateway1`

This directory will be the working directory of the process and also contain an `.rc` configuration file as well as a `.txt` file to capture the `STDOUT` and `STDERR` of the process, like this:

```
gateway.rc
gateway.txt
```

There will also be an XML setup file and so on.