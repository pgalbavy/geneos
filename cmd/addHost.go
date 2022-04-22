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
	Use:     "host NAME [SSHURL]",
	Aliases: []string{"remote"},
	Short:   "Add a remote host",
	Long:    `Add a remote host for integration with other commands. The`,
	Annotations: map[string]string{
		"wildcard": "false",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, args, params := processArgs(cmd)
		if len(args) == 0 {
			return geneos.ErrInvalidArgs
		}
		h := host.Get(host.Name(args[0]))
		if h.Loaded() {
			return fmt.Errorf("host %q already exists", args[0])
		}
		return addHost(h, viper.GetString("defaultuser"), params)
	},
}

func init() {
	addCmd.AddCommand(addHostCmd)
}

//
// 'geneos add remote NAME [SSH-URL] [init opts]'
//
func addHost(r *host.Host, username string, params []string) (err error) {
	if len(params) == 0 {
		// default - try ssh to a host with the same name as remote
		params = []string{"ssh://" + string(r.Name)}
	}

	var remurl string
	if strings.HasPrefix(params[0], "ssh://") {
		remurl = params[0]
		params = params[1:]
	} else if strings.HasPrefix(params[0], "/") {
		remurl = "ssh://" + r.String() + params[0]
		params = params[1:]
	} else {
		remurl = "ssh://" + r.String()
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
	r.V().Set("hostname", u.Host)
	if u.Host == "" {
		r.V().Set("hostname", r.Name)
	}

	if u.Port() != "" {
		r.V().Set("port", u.Port())
	}

	if u.User.Username() != "" {
		username = u.User.Username()
	}
	r.V().Set("username", username)

	// XXX default to remote user's home dir, not local
	r.V().Set("geneos", host.Geneos())
	if u.Path != "" {
		// XXX check and adopt local setting for remote user and/or remote global settings
		// - only if ssh URL does not contain explicit path
		r.V().Set("geneos", u.Path)
	}

	if err = host.WriteConfig(r); err != nil {
		return
	}

	// once we are bootstrapped, read os-release info and re-write config
	if err = r.GetOSReleaseEnv(); err != nil {
		return
	}

	if err = host.WriteConfig(r); err != nil {
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

	// initialise the remote directory structure, but perhaps ignore errors
	// as we may simply be adding an existing installation
	// if err = geneos.Init(r, []string{r.Geneos}); err != nil {
	// 	return err
	// }

	// for _, c := range components {
	// 	if c.Initialise != nil {
	// 		c.Initialise(r)
	// 	}
	// }

	return
}
