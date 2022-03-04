# `geneos` management program

The `geneos` program will help you manage your Geneos environment, one server at a time. Some of it's features, existing and planned, include:

* Set-up a new environment with a series of simple commands
* Remote systems support (very much early testing)
* Add new instances of common componments with sensible defaults
* Check the status of components
* Stop, start, restart components (without minimal delays)
* Support fo existing gatewayctl/netprobectl/licdctl scripts and their file and configuration layout
* Convert existing set-ups to JSON based configs with more options
* Edit individual settings of instances
* Automatically download and install the latest packages (authentication required, for now)
* Update running versions - stop, update, start
* TLS certificate generation and management


## Getting Started

You can either download a sinfle binary - or build and install from source, if you have Go 1.17+ installed:

`go install wonderland.org/geneos/cmd/geneos@latest`

or - if you want the bleeding edge -

`go install wonderland.org/geneos/cmd/geneos@HEAD`

Make sure that the `geneos` program is in your normal `PATH` - or that $HOME/go/bin is if you used the method above - to make things simpler.

### Existing Installation

If you have an existing Geneos installation that you manage with the legacy `gatewayctl` / `netprobectl` commands then you can try these once you have set the geneos directory:

`geneos ls` - list instances
`geneos ps` - show their running status
`geneos show` - show the default configuration values

None of these commands should have any side-effects but others will not only start or stop processes but may also convert configuration files to JSON format without further prompting. Old `.rc` files are backed-up with a `.rc.orig` extension.

Note: You have to set an environment variable `ITRS_HOME` pointing to the top-level directory of an existing installation or set the location in the user or global configuration file:

`geneos set ITRSHome=/path/to/install`

This is the directory where the `packages` and `gateway` (etc.) directories live. If you do not have an existing installation then we initialise one below.

## New Installation

```bash
geneos init
geneos download latest
geneos new gateway Gateway1
geneos new netprobe Netprobe1
geneos new licd Licd1
geneos upload licd Licd1 geneos.lic
geneos start
```

Instance names are case sensitive and cannot be the same as some reserved words (e.g. `gateway`, `netprobe`, `probe` and more, given below).

You still have to configure the Gateway to connect to the Netprobe, but all three components should now be running. You can check with:

`geneos ps`

## Security and Running as Root

This program has been written in such a way that is *should* be safe to install SETUID root or run using `sudo` for almost all cases. The program will refuse to accidentally run an instance as root unless the `User` config parameter is explicitly set - for example when a Netprobe needs to run as root. As with many complex programs, care should be taken and privileged execution should be used when required.

## Remote Management (NEW!)

The `geneos` command can now transparently manage instances across multiple systems using SSH. Some things works well, some work with minor issues and some features do not work at all.

This feature is still very much under development there will be changes coming.

### What does this mean?

See if these commands give you a hint:

```bash
$ geneos add remote server2 ssh://geneos@myotherserver.example.com/opt/geneos
$ geneos add gateway newgateway@server2
$ geneos start
```

Command like `ls` and `ps` will works transparently and merge all instances together, showing you where they are runnng (or not).

A remote is a psuedo-instance and you add and manage it with the normal commands. At the moment the only supported transport is SSH and the URL is a slightly extended version of the RFC standard to include the Geneos home directory. The format, for the `add` command is:

`ssh://[USER@]HOST[:PORT][/PATH]`

If not set, USER defaults to the current username. Similarly PORT defaults to 22. PATH defaults to the local ITRSHome path. The most basic SSH URL of the form `ssh://hostname` results in a remote accessed as the current user on the default SSH port and rooted in the same directory as the local set-up. Is the remote directory is empty (dot files are ignored) then the standard file layout is created.

### How does it work?

There are a number of prerequisites for remote support:

1. Linux on amd64 for all servers
2. Passwordless SSH access, either via an `ssh-agent` or unprotected private keys
3. At this time the only private keys supported are those in your `.ssh` directory beginning `id_` - later updates will allow you to set the name of the key to load, but using an agent is recommended.
4. The remote user must be confiugured to use a `bash` shell or similar. See limitations below.

If you can log in to a remote Linux server using `ssh user@server` and no be prompted for a password or passphrase then you are set to go. It's beyond the scope of this README to explain how to set-up `ssh-agent` or how to create an unprotected private key file, so please search online.

### Limitations

The remote connections over SSH mean there are limitations to the features available on remote servers:

1. No following logs (i.e. the `-f` option). The program is written to use `fsnotify` and that only works on local filesystems and not over sftp. This may be added using a more primitive polling mecahnism later.
2. Control over instance processes is done via shell commands and little error checking is done, so it is possible to cause damage and/or processes not to to start or stop as expected. Contributions of fixes are welcomed.
3. All actions are taken as the user given in the SSH URL (which should NEVER be `root`!) and so instances that are meant to run as other users cannot be controlled. Files and directories may not be available if the user does not have suitable permissions.

## Usage

Please note that the full list of commands and parameters is changing all the time. This list below is mostly, but not completely, up-to-date.

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

#### General Command Flags & Arguments

`geneos [FLAG...] COMMAND [FLAG...] [TYPE] [NAME...] [PARAM...]`

Where:

* `FLAG` - parsed by the flag package
* `COMMAND` - one of the configured command verbs
* `TYPE` - parsed by CompType() where None means no match
* `NAME` - one or more instance names, matching the validNames() test
* `PARAM` - everything else, left after the last NAME is found

Special case or genearlise some commands - the don't call parseArgs() or whatever. e.g. "geneos set global [PARAM...]"

Reserved instance names are case-insensitive. So, for exmaple, "gateway", "Gateway" and "GATEWAY" are all reserved.

#### Global Commands

* `geneos version`
Show the current version of the `geneos` program, which should match the tag of the overall `geneos` package.

* `geneos help`
General help, initially a list of all the supported commands.

* `geneos ls [TYPE] [NAME...]`
Output a list of all configured instances. If a TYPE and/or NAME(s) are supplied then list those that match.

* `geneos ps [TYPE] [NAME...]`
Show details of running instances.

* `geneos logs [-f | -n N | ...] [TYPE] [NAME...]`
Show log(s) for matching instances. Flags allow for follow etc.

#### Environment Commands

* `geneos init [username] [directory]`
Initialise the environment. 

* `geneos tls ...`
TLS operations. See below.

* `geneos show [global|user]`
Show the running configuration or, if `global` or `user` is supplied then the respective on-disk configuration files. Passwords are simplisticly redated.
The instance specific `show` command is described below.

* `geneos set [global|user] KEY=VALUE...`
Set a program-wide configuration option. The default is to update the `user` configuration file. If `global` is given then the user has to have appropriate privileges to write to the global config file (`/etc/geneos/geneos.json`). Multiple KEY=VALUE pairs can be given but only fields that are recognised are updated.
The instance specific version of the `set` command is described below.
#### Package Managemwent Commands

* `geneos extract [FILE...]`
Extracts a local release archive into the `packages` directory.

* `geneos download [TYPE] [lastest|URL...]`
Download and install a release archive in the `packages` directory.

* `geneos update [TYPE] [VERSION]`
Update the component base binary link

#### Instance Control Commands

* `geneos start [-l] [TYPE] [NAME...]`
Start a Geneos component. If no name is supplied or the special name `all` is given then all the matching Geneos components are started.

* `geneos stop [-f] [TYPE] [NAME...]`
Like above, but stops the component(s)
-f terminates forcefully - i.e. a SIGKILL is immediately sent

* `geneos restart [-l] [TYPE] [NAME...]`
Restarts matching geneos components. Each component is stopped and started in sequence. If all components should be down before starting up again then use a combination of `start` and `stop` from above.

* `geneos reload|refresh [TYPE] NAME [NAME...]`
Signal the component to reload it's configuration or restart as appropriate.

* `geneos disable [TYPE] [NAME...]`
Stop and disable the selected compoents by placing a file in wach working directory with a `.disable` extention

* `geneos enable [TYPE] [NAME...]`
Remove the `.disable` lock file and start the selected components

* `geneos clean [-f] [TYPE] [names]`
Clean up component directory. Optionally 'full' clean, with an instance restart.

#### Instance Configuration Commands

* `geneos new [TYPE] name [NAME...]`
Create a Geneos component configuration.

* `geneos migrate [TYPE] [NAME...]`
Migrate legacy `.rc` files to `.json` and backup the original file with an `.orig` extension. This backup file can be used by the `revert` command, below, to restore the original `.rc` file(s)

* `geneos revert [TYPE] [NAME...]`
Revert to the original configuration files, deleting the `.json` files. Note that the `.rc` files are never changed and any configuration changes to the `.json` configuration will not be retained.

* `geneos command [TYPE] [NAME...]`
Shows details of the full command used for the component and any extra environment variables found in the configuration.

* `geneos rename [TYPE] name newname`
Rename the compoent, but this only affects the container directory, this programs JSON configursation file and does not update the contents of any other files.

* `geneos delete component name`
Deletes the disabled component given. Only works on components that have been disabled beforehand.

* `geneos edit [user|component] [names]`
Open an editor for the selected instances or user JSON config file. Will accept wild or multiple instance names.

* `geneos upload [TYPE] name [file|url|-]`
Upload a file into an instance working directory, from local file, url or stdin and backup previous file. The file can also specify the destination name and sub-directory, which will be created if it does not exist. Examples of valid files are:
  ```bash
  geneos upload gateway Example gateway.setup.xml
  geneos upload gateway Example https://server/files/gateway.setup.xml
  geneos upload gateway Example gateway.setup.xml=mygateway.setup.xml
  geneos upload gateway Example scripts/newscript.sh=myscript.sh
  geneos upload gateway Example scripts/=myscript.sh
  cat someoutput | geneos upload gateway Example config.json=-
  ```

Like other commands that write to the file system is can safely be run as root as the destination directory and file will be changed to be owned by either the instance or the default user, with the caveat that any intermediate directrories above the destination sub-directory (e.g. the first two in `my/long/path`) will be owned by root.
`geneos clean` will remove backup files. Principal use for license token files, XML configs, scripts.

## TLS Operations

The `geneos tls` command provides a number of subcommands to create and manage certificates and instance configurations for encrypted connections.

Once enabled then all new instances will also have certificates created and configuration set to use secure (encrypted) connections where possible.

* `geneos tls init`
  Initialised the TLS environment by creating a `tls` directory in ITRSHome and populkating it with a new root and intermediate (signing) certificate and keys as well as a `chain.pem` which includes both CA certificates. The keys are only readable by the user running the command. Also does a `sync` if remotes are configured.

  Any existing instances have certificates created and their configurations updated to reference them. This means that any legacy `.rc` configurations will be migrated to `.json` files.

* `geneos tls import FILE [FILE...]`
  Import certificates and keys as specified to the `tls` directory as signing certificate and key. If both are in the same file then they are split into a certificate and key and he key file is permissioned so that it is only accessible to the user running the command. [Note: This is currently untested].

* `geneos tls new [TYPE] [NAME...]`
  Create a new certificate for matching instances, siogned using the signing certificate and key. This will NOT overwrite an existing certificate and will re-use the private key if it exists. The default validity period is one year. This cannot currently be changed.

* `geneos tls renew [TYPE] [NAME...]`
  Renew a certificate for matching instances. This will overwrite an existing certificate regardless of it's current status of validity period. There must already be a private key.

* `geneos tls ls [-c | -j | -i | -l] [TYPE] [NAME...]`
  List instance certificate information. Options are the same as for the main `ls` command but the data shown is specific to certificates.

* `geneos tls sync`
  Copies chain.pem to all remotes

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
 