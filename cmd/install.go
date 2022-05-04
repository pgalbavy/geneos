/*
Copyright © 2022 Peter Galbavy <peter@wonderland.org>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install [-b BASENAME] [-l] [-n] [-r REMOTE] [-U] [-T TYPE:VERSION] [TYPE] | FILE|URL [FILE|URL...] | [VERSION | FILTER]",
	Short: "Install files from downloaded Geneos packages. Intended for sites without Internet access",
	Long: `Installs files from FILE(s) in to the packages/ directory. The filename(s) must of of the form:

	geneos-TYPE-VERSION*.tar.gz

The directory for the package is created using the VERSION from the archive
filename unless overridden by the -T and -V flags.

If a TYPE is given then the latest version from the packages/downloads
directory for that TYPE is installed, otherwise it is treated as a
normal file path. This is primarily for installing to remote locations.

TODO:

Install only changes creates a base link if one does not exist.
To update an existing base link use the -U option. This stops any
instance, updates the link and starts the instance up again.

Use the update command to explicitly change the base link after installation.

Use the -b flag to change the base link name from the default 'active_prod'. This also
applies when using -U.

"geneos install gateway"
"geneos install fa2 5.5 -U"
"geneos install netprobe -b active_dev -U"
"geneos update gateway -b active_prod"

`,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "false",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ct, args, params := cmdArgsParams(cmd)
		return commandInstall(ct, args, params)
	},
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().StringVarP(&installCmdBase, "base", "b", "active_prod", "Override the base active_prod link name")

	installCmd.Flags().BoolVarP(&installCmdLocal, "local", "l", false, "Install from local files only")
	installCmd.Flags().BoolVarP(&installCmdNoSave, "nosave", "n", false, "Do not save a local copy of any downloads")
	installCmd.Flags().StringVarP(&installCmdRemote, "remote", "r", string(host.ALLHOSTS), "Perform on a remote. \"all\" means all remotes and locally")

	installCmd.Flags().BoolVarP(&installCmdUpdate, "update", "U", false, "Update the base directory symlink")
	installCmd.Flags().StringVarP(&installCmdOverride, "override", "T", "", "Override (set) the TYPE:VERSION for archive files with non-standard names")
	installCmd.Flags().SortFlags = false
}

var installCmdLocal, installCmdNoSave, installCmdUpdate bool
var installCmdBase, installCmdRemote, installCmdOverride string

func commandInstall(ct *geneos.Component, args, params []string) (err error) {
	// first, see if user wants a particular version
	version := "latest"

	for n := 0; n < len(args); n++ {
		if geneos.MatchVersion(args[n]) {
			version = args[n]
			args[n] = args[len(args)-1]
			args = args[:len(args)-1]
		}
	}

	h := host.Get(host.Name(installCmdRemote))

	// if we have a component on the command line then use an archive in packages/downloads
	// or download from official web site unless -l is given. version numbers checked.
	// default to 'latest'
	//
	// overrides do not work in this case as the version and type have to be part of the
	// archive file name
	if ct != nil {
		logDebug.Printf("installing %q version of %s to %s remote(s)", version, ct, installCmdRemote)
		f, r, err := geneos.OpenArchive(host.LOCAL, ct, geneos.Version(version), geneos.LocalOnly(installCmdLocal))
		if err != nil {
			return err
		}
		defer r.Close()

		if h == host.ALL {
			for _, h := range host.AllHosts() {
				if err = geneos.MakeComponentDirs(h, ct); err != nil {
					return err
				}
				if err = geneos.Unarchive(h, ct, f, r, geneos.Basename(installCmdBase), geneos.Overwrite(installCmdUpdate), geneos.Override(installCmdOverride)); err != nil {
					logError.Println(err)
					continue
				}
			}
		} else {
			if err = geneos.MakeComponentDirs(h, ct); err != nil {
				return err
			}
			if err = geneos.Unarchive(h, ct, f, r, geneos.Basename(installCmdBase), geneos.Overwrite(installCmdUpdate), geneos.Override(installCmdOverride)); err != nil {
				return err
			}
			logDebug.Println("installed", ct.String())
		}

		return nil
	}

	// no component type means we might want file or url or auto url
	if len(args) == 0 {
		// normal download here
		if installCmdLocal {
			log.Println("install -l (local) flag with no component or file/url")
			return nil
		}
		var rs []*host.Host
		if installCmdRemote == string(host.ALLHOSTS) {
			rs = host.AllHosts()
		} else {
			rs = []*host.Host{host.Get(host.Name(installCmdRemote))}
		}

		for _, r := range rs {
			if err = geneos.MakeComponentDirs(r, ct); err != nil {
				return err
			}
			// if err = geneos.Download(r, ct, version, installCmdBase, installCmdUpdate); err != nil {
			if err = geneos.Download(r, ct, geneos.Version(version), geneos.Basename(installCmdBase), geneos.Overwrite(installCmdUpdate), geneos.LocalOnly(installCmdLocal)); err != nil {
				logError.Println(err)
				continue
			}
		}
		// Download() in the loop above calls updateToVersion()
		return nil
	}

	// work through command line args and try to install them using the naming format
	// of standard downloads - fix versioning
	for _, file := range args {
		f, filename, err := geneos.OpenLocalFileOrURL(file)
		if err != nil {
			log.Println(err)
			continue
		}
		defer f.Close()

		if installCmdRemote == string(host.ALLHOSTS) {
			for _, r := range host.AllHosts() {
				// what is finalVersion ?
				if err = geneos.MakeComponentDirs(r, ct); err != nil {
					return err
				}
				if err = geneos.Unarchive(r, ct, filename, f, geneos.Basename(installCmdBase), geneos.Overwrite(installCmdUpdate), geneos.Override(installCmdOverride)); err != nil {
					logError.Println(err)
					continue
				}
			}
		} else {
			r := host.Get(host.Name(installCmdRemote))
			geneos.MakeComponentDirs(r, ct)
			if err = geneos.Unarchive(r, ct, filename, f, geneos.Basename(installCmdBase), geneos.Overwrite(installCmdUpdate), geneos.Override(installCmdOverride)); err != nil {
				return err
			}
			logDebug.Println("installed", ct.String())
		}
	}

	return nil
}
