# `geneosctl`

The `geneosctl` program will help you manage your Geneos components. Some of it's features, existing and planned, include:

* Set-up a new environment with a single command
* Add new instances of common componments with sensible defaults
* Check the status of components
* Stop, start, restart components (without those unnecessary delays from `sleep` in traditional shell scripts)
* Support fo existing gatewayctl/netprobectl/licdctl scripts and their file and configuration layout
* Convert existing set-ups to JSON based configs with more options

* 

## Usage

Either download the standalone binary or build from source:

```
git clone https://github.com/pgalbavy/geneos.git
cd geneos/cmd/geneosctl
go build
- or -
go install
```

Make sure that the `geneosctl` program is in your normal `PATH` to make execution easier. You should be able to immediately try:

`geneosctl list` - show all components
`geneosctl status` - show their running status

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


