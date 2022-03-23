# `geneos` management program

The `geneos` program will help you manage your Geneos environment.

You can:

* Initialise a new installation in one command
* Adopt an existing installation that uses older tools
* Manage a group of servers with a single command
* Manage certificates for TLS connectivity between Geneos components
* Configure the environment of components without editing files
* Download and install Geneos software, update components
* Simple bootstrapping of Self-Announcing Netprobes

The aim is to:

* Keep it simple - "The law of least astonishment"
* Make your life easier - at least the part managing Geneos
* Help you use other automation tools with Geneos

## Getting Started

### Download the binary

You can download a pre-built binary version (for Linux on amd64 only):

```bash
# edit version to suit - until this is a non pre-release 'latest' will not work
curl -OL https://github.com/pgalbavy/geneos/releases/download/v0.17.0/geneos
chmod +x geneos
sudo mv geneos /usr/local/bin/
```

### Build from source

To build from source you have Go 1.17+ installed:

#### One line installation

    go install wonderland.org/geneos/cmd/geneos@latest

Make sure that the `geneos` program is in your normal `PATH` - or that $HOME/go/bin is if you used the method above - to make things simpler.

#### Download from github and build manually

Make sure you do not have an existing file or directory called `geneos` and then:

```bash
github clone https://github.com/pgalbavy/geneos.git
cd geneos/cmd/geneos
go build
sudo mv geneos /usr/local/bin
```

### Adopting An Existing Installation

If you have an existing Geneos installation that you manage with the command like `gatewayctl`/`netprobectl`/etc. then you can use `geneos` to manage those once you have configured the path to the Geneos installation.

You can use an environment variable `ITRS_HOME` pointing to the top-level directory of your installation or set the location in the (user or global) configuration file:

    geneos set ITRSHome=/path/to/install

The directory is where the `packages` and `gateway` (etc.) directories live. If you do not have an existing installation that follows this pattern then you can create a fresh layout further below.

You can now check your installation with some simple commands:

    geneos ls - list instances
    geneos ps - show their running status
    geneos show - show the default configuration values

None of these commands should have any side-effects but others will not only start or stop processes but may also convert configuration files to JSON format without further prompting. Old `.rc` files are backed-up with a `.rc.orig` extension.


### New Installation

#### Demo Gateway

You can set-up a Demo environment like this:

    geneos init -d /path/to/empty/dir

This one line command will create a directory structure, download software and configure a Gateway in 'Demo' mode plus a single Netprobe and Webserver for dashboards. However, no configuration is done to connect these up, that's up to you!

Under-the-hood, the command does exactly this for you:

    geneos init /path/to/empty/dit
    geneos download 
    geneos add gateway 'Demo Gateway'
    geneos add netprobe localhost
    geneos add webserver demo
    geneos start
    geneos ps

#### Self-Announcing Netprobe

You can install a configured and running Self-Announcing Netprobe (SAN) in one line, like this:

    geneos init -S -n SAN123 -c /path/to/signingcertkey \
      Gateways=gateway1,gateway2 Types=Infrastructure,App1,App2 \
      Attribuutes=ENVIRONMENT=Prod,LOCATION=London

This example will create a SAN with the name SAN123 connecting to 

#### Another Initial Environment

    geneos init -a geneos.lic

does this (where HOSTNAME is, of course, replaced with the hostname of the server)

```bash
geneos init
geneos download latest
geneos new gateway HOSTNAME
geneos new netprobe HOSTNAME
geneos new licd HOSTNAME
geneos new webserver HOSTNAME
geneos import licd HOSTNAME geneos.lic
geneos start
```

Instance names are case sensitive and cannot be the same as some reserved words (e.g. `gateway`, `netprobe`, `probe` and more, given below).

You still have to configure the Gateway to connect to the Netprobe, but all three components should now be running. You can check with:

`geneos ps`

## Security and Running as Root

This program has been written in such a way that is *should* be safe to install SETUID root or run using `sudo` for almost all cases. The program will refuse to accidentally run an instance as root unless the `User` config parameter is explicitly set - for example when a Netprobe needs to run as root. As with many complex programs, care should be taken and privileged execution should be used when required.

## Instance Settings

### `Env=` parameters

All instances support customer environment variables being set or unset. This is done through the `set` command below, alongside the standard configuration parameters for ech instance type.

To set an environment variable use this syntax:

    geneos set netprobe example1 Env=PATH_TO_SOMETHING=/this/way

If an entry already exists it is overwritten.

To remove an entry, prefix the name with a minus (`-`) sign, e.g.

    geneos set netprobe examples1 Env=-PATH_TO_SOMETHING

You can also remove multiple entries with a very simply wildcard syntax `NAME*` - this is only supported as the last character of the name and will remove all entries that start with `NAME` in this case.

You can specify multiple entries by comma seperating them:

    geneos set netprobe example1 Env=JAVA_HOME=/path,ORACLE_HOME=/path2

Note that this means you cannot insert commas into values as there is no supported escape mecahnism. For this you must edit the instance configuration file directly, which you can also do with the command `edit`.

Finally, if your environment variable value contains spaces then use quotes as appropriate to your shell to prevent those spaces being processed. In bash you can do any of these to achieve the same result:

    geneos set netprobe example1 Env=MYVAR="a string with spaces"
    geneos set netprobe example1 Env="MYVAR=a string with spaces"
    geneos set netprobe example1 "Env=MYVAR=a string with spaces"

You can review the environment for any instance using the `show` command:

    geneos show netprobe example1
  
A more human readable output is available from the `command` command:

    geneos command netprobe example1

There are similar meta-parameters available for some specific component types. These are documented below.

## Component Types

The following component types (and their aliases) are supported:

* `any` or empty
* `gateway`, `gateways`
* `netprobe`, `netprobes`, `probe` or `probes`
* `licd` or `licds`
* `webserver`, `webservers`, `webdashboard`. `dashboards`
* `san`, `sans`
* `fa2`, `fixanalyser`, `fix-analyser`
* `fileagent`, `fileagents`

These names are also reserved words and you cannot configure or manage components with those names. This means that you cannot have a gateway called `gateway` or a probe called `netprobe`. If you do already have instances with these names then you will have to be careful migrating. See more below.

Each component type is described below along with specific component options.

### Type `gateway`

* Gateway general
* Gateway templates
When creating a new Gateway instance a default `gateway.setup.xml` file is created from the template(s) installed in the `gateway/templates` directory. By default this file is only created once but can be re-created using the `rebuild` command with the `-f` option if required. In turn this can also be protected against by setting the Gateway configuration setting `ConfigRebuild` to `never`.
* Gateway variables for templates
Gateways support the setting of Include file parameters for use in templated configurations. These are set similarly to the general `Env=` parameters above but follow a slighty different syntax:
  `geneos gateway set example2 Includes=100:/path/to/include`
The setting value is `priority:path` and path can be a realtive or absolute path or a URL. In the case of a URL the source is NOT downloaded but instead the URL is inserted as-is in the templates.
As for `Env=` entries can be removed with a minus (`-`) prefix but no wilcarding is allowed. Comma seperated lists work as normal.


### Type `netprobe`

* Netprobe general

### Type `licd`


### Type `webserver`

* Webserver general
* Java considerations
* Configuration templates - TBD

### Type `san`

* San general
* San templates
* San variables for templates
As for Gateways, Sans get a default confiuration file when they are created. By default this is from the template(s) in `san/templates`. Unlike for the Gateway these configuration files are rebuilt by the `rebuild` command by default. This allows the administrator to maintain Sans using only command line tools and avoimd having to edit XML directly.
To aid this, Sans support the following parameters, similar to `Env=` above:
  * Attributes
  Attribute follow the same KEY=VALUE settings and `Env` above
  * Gateways
  Gateways are configured using the syntax `hostname:port`
  * Types
  Types are a simple list of comma seperated names for Types. They can be removes using a minux (`-`) as for `Env` but do not allow wildcards
  * Variables
  Variables are not yet supported from the command line but can be set by editing the San instance configuration file. This is because Variables have a type as well as a name and a value, so this need more work.
* Selecting the underlying Netprobe type (For Fix Analyser 2 below)
A San instance will normally be built to use the general purpose Netprobe package. To use an alternative package, such as the Fix Analyser 2 Netprobe, add the instance with the special format name `fa2:example[@REMOTE]` - this configures the instance to use the `fa2` as the underlying package. Any future special purpose Netprobes can also be supported in this way.

### Type `fa2`

* Fix Analyser 2 general

### Type `fileagent`

* File Agent general

## Remote Management

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

1. Remote servers must be Linux on amd64
2. Passwordless SSH access, either via an `ssh-agent` or unprotected private keys
3. At this time the only private keys supported are those in your `.ssh` directory beginning `id_` - later updates will allow you to set the name of the key to load, but using an agent is recommended.
4. The remote user must be confiugured to use a `bash` shell or similar. See limitations below.

If you can log in to a remote Linux server using `ssh user@server` and no be prompted for a password or passphrase then you are set to go. It's beyond the scope of this README to explain how to set-up `ssh-agent` or how to create an unprotected private key file, so please search online.

### Limitations

The remote connections over SSH mean there are limitations to the features available on remote servers:

1. No following logs (i.e. the `-f` option). The program is written to use `fsnotify` and that only works on local filesystems and not over sftp. This may be added using a more primitive polling mecahnism later.
2. Control over instance processes is done via shell commands and little error checking is done, so it is possible to cause damage and/or processes not to to start or stop as expected. Contributions of fixes are welcomed.
3. All actions are taken as the user given in the SSH URL (which should NEVER be `root`) and so instances that are meant to run as other users cannot be controlled. Files and directories may not be available if the user does not have suitable permissions.



## Usage

CAUTION: Please note that the full list of commands and parameters is still changing at this time. This list below is mostly, but not completely, up-to-date.

The general syntax is:

`geneos COMMAND [TYPE] [NAMES...]`

There are a number of special cases, these are detailed below.


### Commands

#### General Command Flags & Arguments

`geneos [FLAG...] COMMAND [FLAG...] [TYPE] [NAME...] [PARAM...]`

Where:

* `FLAG` - Both general and command specific flags
* `COMMAND` - one of the configured commands
* `TYPE` - the compone type
* `NAME` - one or more instance names, optionally including the remote server
* `PARAM` - anything that isn't one of the above

In general, with the exception of `TYPE`, all parameters can be in any order as they are filtered into their types for most commands. Some commands require arguments in an exact order. For example, these should be treated the same way:

`geneos ls -c gateway one two three`
`geneos ls gateway one -c two three`

Reserved instance names are case-insensitive. So, for exmaple, "gateway", "Gateway" and "GATEWAY" are all reserved.

The `NAME` is of the format `INSTANCE@REMOTE` where either is optional. In general commands will wildcard the part not provided. There are special `REMOTE` names `@local` and `@all` - the former is, as the name suggests, the local server and `@all` is the same as not providing a remote name.

There is a special format for adding Sans in the form `TYPE:NAME@REMOTE` where `TYPE` can be used to select the underlying Netprobe type. This format is still accepted for all other commands but the `TYPE` is completely ignored.

#### File and URLs

In general all source file references support URLs, e.g. imported certificate and keys, license files, importing general files.

The primary exception is for Gateway include files used in templated configurations. If these are given as URLs then they are used in the configuration as URLs.

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

* `geneos init [-T|-S|-D|-A LICENSE] [-c CERT] [-k KEY] [-n NAME] [USERNAME] [PATH] [PARAMS]`
Initialise the environment. This command creates a directory hierarchy and optionally downloads and extracts Geneos software packages and also optionally creates instances and starts them.
  * `-T` Rebuild the default templates using the embedded files. This is primarily to update tempates when new versions of this program are release or if they have become corrupted
  * `-S` Build and start a San. See the `-n` option below. Takes all the same PARAMS as for adding a San to specify template settings.
  * `-D` Build and start a demo environment
  * `-A LICENSE` Build and start an `all` environment
  * `-c CERT` and `-k KEY` Import certificates and keys during initialisation. See `geneos tls import` for more details. When a valid signing certificate and key are imported then all subsequent new instances will have individual certificates and keys created.
  * `-n NAME` Use the `NAME` for instances instead of the default hostname. This is especially usefult for Sans and Gateways as the templates use this name to fill in various confiruation item defaults
  * `-s FILE` A San template file to use instead of the embedded one
  * `-g FILE` A Gateway template file to use instead of the embedded one
Only one of the `-t`, `-S`, `-d` or `-a` options are valid and only the `-t` option can be used for multiple calls to this command.

* `geneos tls ...`
TLS operations. See below.

* `geneos show [global|user]`
Show the running configuration or, if `global` or `user` is supplied then the respective on-disk configuration files. Passwords are simplisticly redated.
The instance specific `show` command is described below.

* `geneos set [global|user] KEY=VALUE...`
Set a program-wide configuration option. The default is to update the `user` configuration file. If `global` is given then the user has to have appropriate privileges to write to the global config file (`/etc/geneos/geneos.json`). Multiple KEY=VALUE pairs can be given but only fields that are recognised are updated.

* `geneos home [TYPE] [NAME]`
The `home` command outputs the home directory of the first matching instance, or `ITRSHome` if there is no match or no options passed to the command. This is useful for automation and shortcuts, w.g. in bash:

    $ cd $(geneos home netprone example1)

Please note that if `geneos home` returns an empty string because of an error the cd command will take you to your home directory.

#### Package Management Commands

* `geneos extract [FILE...]`
Extracts a local release archive into the `packages` directory.

* `geneos download [TYPE] [lastest|URL...]`
Download and install a release archive in the `packages` directory.

* `geneos update [TYPE] [VERSION]`
Update the component base binary link

#### Control Commands

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

#### Configuration Commands

* `geneos add [TYPE] NAME [NAME...]`
Add a new Geneos component configuration.

* `geneos migrate [TYPE] [NAME...]`
Migrate legacy `.rc` files to `.json` and backup the original file with an `.orig` extension. This backup file can be used by the `revert` command, below, to restore the original `.rc` file(s)

* `geneos revert [TYPE] [NAME...]`
Revert to the original configuration files, deleting the `.json` files. Note that the `.rc` files are never changed and any configuration changes to the `.json` configuration will not be retained.

* `geneos rebuild [-n] [-f] [TYPE] [NAME...]`
Rebuild instance configuration, typically used for Self-Announcing Netprobes. By default it restarts any instances where the configuration has changed. Flags are:
  * `-n` Do not restart instances
  * `-f` Force rebuild for those instances that are maked `initial` only.

* `geneos command [TYPE] [NAME...]`
Shows details of the full command used for the component and any extra environment variables found in the configuration.

* `geneos rename [TYPE] name newname`
Rename the compoent, but this only affects the container directory, this programs JSON configursation file and does not update the contents of any other files.

* `geneos delete component name`
Deletes the disabled component given. Only works on components that have been disabled beforehand.

* `geneos edit [user|component] [names]`
Open an editor for the selected instances or user JSON config file. Will accept wild or multiple instance names.

* `geneos import [TYPE] name [file|url|-]`
Import a file into an instance working directory, from local file, url or stdin and backup previous file. The file can also specify the destination name and sub-directory, which will be created if it does not exist. Examples of valid files are:
  ```bash
  geneos import gateway Example gateway.setup.xml
  geneos import gateway Example https://server/files/gateway.setup.xml
  geneos import gateway Example gateway.setup.xml=mygateway.setup.xml
  geneos import gateway Example scripts/newscript.sh=myscript.sh
  geneos import gateway Example scripts/=myscript.sh
  cat someoutput | geneos import gateway Example config.json=-
  ```

Like other commands that write to the file system is can safely be run as root as the destination directory and file will be changed to be owned by either the instance or the default user, with the caveat that any intermediate directrories above the destination sub-directory (e.g. the first two in `my/long/path`) will be owned by root.
`geneos clean` will remove backup files. Principal use for license token files, XML configs, scripts.

## TLS Operations

The `geneos tls` command provides a number of subcommands to create and manage certificates and instance configurations for encrypted connections.

Once enabled then all new instances will also have certificates created and configuration set to use secure (encrypted) connections where possible.

The root and signing certificates are only kept on the local server and the `tls sync` command can be used to copy a `chain.pem` file to remote servers. Keys are never copied to remote servers by any built-in commands.

* `geneos tls init`
  Initialised the TLS environment by creating a `tls` directory in ITRSHome and populating it with a new root and intermediate (signing) certificate and keys as well as a `chain.pem` which includes both CA certificates. The keys are only readable by the user running the command. Also does a `sync` if remotes are configured.

  Any existing instances have certificates created and their configurations updated to reference them. This means that any legacy `.rc` configurations will be migrated to `.json` files.

* `geneos tls import FILE [FILE...]`
  Import certificates and keys as specified to the `tls` directory as root or signing certificates and keys. If both certificate and key are in the same file then they are split into a certificate and key and the key file is permissioned so that it is only accessible to the user running the command.
  Root certificates are identified by the Subject being the same as the Issuer, everything else is treated as a signing key. If multiple certificates of the same type are imported then only the last one is saved. Keys are checked against certificates using the Public Key part of both and only complete pairs are saved.

* `geneos tls new [TYPE] [NAME...]`
  Create a new certificate for matching instances, signed using the signing certificate and key. This will NOT overwrite an existing certificate and will re-use the private key if it exists. The default validity period is one year. This cannot currently be changed.

* `geneos tls renew [TYPE] [NAME...]`
  Renew a certificate for matching instances. This will overwrite an existing certificate regardless of it's current status of validity period. Any existing provate key will be re-used. `renew` can be used after `import` to create certificates for all instances, but if you already have specfic instance certificates in place you should use `new` above.
  As for `new` the validity period is a year and cannot be changed at this time.

* `geneos tls ls [-a] [-c|-j] [-i] [-l] [TYPE] [NAME...]`
  List instance certificate information. Flags are similar as for the main `ls` command but the data shown is specific to certificates. Additional flags are:
  * `-a` List all certificates. By default the root and signing certificates are not shown
  * `-l` Long list format, which includes the Subject and Signature. This signature can be used directly in the Geneos Authentication entry for users for non-user authentication using client certificates, e.g. Gateway Sharing and Web Server.

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
 