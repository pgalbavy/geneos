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
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/pkg/logger"
)

// give these more convenient names and also shadow the std log
// package for normal logging
var (
	log      = logger.Log
	logDebug = logger.Debug
	logError = logger.Error
)

var (
	ErrInvalidArgs  error = errors.New("invalid arguments")
	ErrNotSupported error = errors.New("not supported")
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "geneos",
	Short: "Control your Geneos environment",
	Long: `Control your Geneos environment. With 'geneos' you can initialise
a new installation, add and remove components, control processes and build
template based configuration files for SANs and new gateways.`,
	SilenceUsage: true,
	Annotations:  make(map[string]string),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		// check initialisation
		geneosdir := host.Geneos()
		if geneosdir == "" {
			// only allow init through
			if cmd != initCmd {
				cmd.SetUsageTemplate(" ")
				return fmt.Errorf("%s", `Installation directory is not set.

You can fix this by doing one of the following:

1. Create a new Geneos environment:

	$ geneos init

	or, if not in your home directory:

	$ geneos init /path/to/geneos

2. Set the ITRS_HOME environment:

	$ export ITRS_HOME=/path/to/geneos

3. Set the Geneos path in your user's configuration file:

	$ geneos set user Geneos=/path/to/geneos

3. Set the Geneos path in the global configuration file (usually as root):

	# echo '{ "Geneos": "/path/to/geneos" }' > `+geneos.GlobalConfig)
			}
		}

		parseArgs(cmd, args)
		return
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var debug, quiet bool

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "G", "", "config file (defaults are $HOME/.config/geneos.json, "+geneos.GlobalConfig+")")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable extra debug output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet mode")
	rootCmd.Flags().MarkHidden("debug")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if quiet {
		log.SetOutput(ioutil.Discard)
	} else if debug {
		logger.EnableDebugLog()
	}

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserConfigDir()
		cobra.CheckErr(err)

		// Search config in home directory with name "geneos" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath("/etc/geneos")
		viper.SetConfigType("json")
		viper.SetConfigName("geneos")
	}

	// u, _ := user.Current()
	// viper.SetDefault("defaultuser", u.Name)
	viper.BindEnv("geneos", "ITRS_HOME")
	viper.AutomaticEnv()
	viper.ReadInConfig()

	// initialise after config loaded
	host.Init()
}
