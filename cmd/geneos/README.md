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

Make sure that the `geneos` program is in your normal `PATH` to make execution easier. You should be able to immediately try:

`geneos list` - show all components
`geneos status` - show their running status

Neither of these commands should have any side-effects but others will not only start or stop processes but may also convert configuration files to JSON format without further prompting. Old `.rc` files are backed-up with a `.rc.orig` extension.

### Directory Layout

The environment variable `ITRS_HOME` (default `/opt/itrs`) points to the base directory for all subsequent operations. The basic layout follows that of the original `gatewayctl` etc. including:

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

## Usage

The following component types (and their aliases) are supported:

* `any` or empty
* `gateway` or `gateways`
* `netprobe`, `netprobes`, `probe` or `probes`
* `licd` or `licds`
* `webserver`, `webservers`, `webdashboard`. `dashboards`

The above names are also treated as reserved words and you cannot configure or manage components with those names. This means that you cannot have a gateway called `gateway` or a probe called `netprobe`. This may cause some issues migrating existing installations. See below for more. 

The following commands are supported. Each command may treat arguments differently.

### `geneos list [component]`

Output a list of all configured components. If a compoent is supplied then restrict the list to that particular type.

### `geneos status [component] [names...]`

As above but show the running status of each matching component. If no names are given than all components are shown.

### `geneos version`

Show the current version of the `geneos` program, which should match the tag of the overall `geneos` package.

### `geneos help`

General help, initially a list of all the supported commands.

### `geneos start [component] [name...]`

Start a Geneos component. If no name is supplied or the special name `all` is given then all the matching Geneos components are started.

### `geneos stop [component] [name...]`

Like above, but stops the component(s)

### `geneos kill [component] [name...]` - Not yet

Stops components ungracefully (SIGKILL is sent)

### `geneos restart [component] [name...]`

Restarts matching geneos components. Each component is stopped and started in sequence. If all components should be down before starting up again then use a combination of `start` and `stop` from above.

### `geneos command [component] [name...]`

Shows details of the command syntax used for the component and any extra environment variables found in the configuration.

### `geneos create [component] name [name...]` - Not yet

Create a Geneos component configuration.

### `geneos reload|refresh [component] name [name...]`



## Configfuration Files

