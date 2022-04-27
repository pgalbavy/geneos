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
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
)

// addHostCmd represents the addRemote command
var addHostCmd = &cobra.Command{
	Use:                   "host [-I] NAME [SSHURL]",
	Aliases:               []string{"remote"},
	Short:                 "Add a remote host",
	Long:                  `Add a remote host for integration with other commands.`,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Annotations: map[string]string{
		"wildcard": "false",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, args, params := processArgs(cmd)
		if len(args) == 0 {
			return geneos.ErrInvalidArgs
		}
		logDebug.Println(args[0])
		h := host.Get(host.Name(args[0]))
		if h.Loaded() {
			return fmt.Errorf("host %q already exists", args[0])
		}
		return addHost(h, viper.GetString("defaultuser"), params)
	},
}

func init() {
	addCmd.AddCommand(addHostCmd)

	addHostCmd.Flags().BoolVarP(&addHostCmdInit, "init", "I", false, "Initialise the remote host directories and component files")
	addHostCmd.Flags().SortFlags = false
}

var addHostCmdInit bool

func addHost(h *host.Host, username string, params []string) (err error) {
	if len(params) == 0 {
		// default - try ssh to a host with the same name as remote
		params = []string{"ssh://" + string(h.Name)}
	}

	var remurl string
	h.V().SetDefault("port", 22)

	if strings.HasPrefix(params[0], "ssh://") {
		remurl = params[0]
		params = params[1:]
	} else if strings.HasPrefix(params[0], "/") {
		remurl = "ssh://" + h.String() + params[0]
		params = params[1:]
	} else {
		remurl = "ssh://" + h.String()
	}

	// if err = initFlagSet.Parse(params); err != nil {
	// 	return
	// }

	u, err := url.Parse(remurl)
	if err != nil {
		return
	}

	if u.Scheme != "ssh" {
		return fmt.Errorf("unsupported scheme (only ssh at the moment): %q", u.Scheme)
	}

	// if no hostname in URL fall back to remote name (e.g. ssh:///path)
	h.V().Set("hostname", u.Host)
	if u.Host == "" {
		h.V().Set("hostname", h.Name)
	}

	if u.Port() != "" {
		h.V().Set("port", u.Port())
	}

	if u.User.Username() != "" {
		username = u.User.Username()
	}
	h.V().Set("username", username)

	// XXX default to remote user's home dir, not local
	h.V().Set("geneos", host.Geneos())
	if u.Path != "" {
		// XXX check and adopt local setting for remote user and/or remote global settings
		// - only if ssh URL does not contain explicit path
		h.V().Set("geneos", u.Path)
	}

	if err = host.WriteConfig(h); err != nil {
		return
	}

	// once we are bootstrapped, read os-release info and re-write config
	if err = h.GetOSReleaseEnv(); err != nil {
		return
	}

	if err = host.WriteConfig(h); err != nil {
		return
	}

	// apply any extra args to settings
	// if len(params) > 0 {
	// 	if err = commandSet(Remote, []string{r.String()}, params); err != nil {
	// 		return
	// 	}
	// 	r.Unload()
	// 	r.Load()
	// }

	if addHostCmdInit {
		// initialise the remote directory structure, but perhaps ignore errors
		// as we may simply be adding an existing installation
		if err = geneos.Init(h, addHostCmdInit, []string{h.V().GetString("geneos")}); err != nil {
			return
		}
	}

	return
}
