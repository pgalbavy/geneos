package main

import (
	"errors"
	"path/filepath"
	"strconv"
)

type Netprobes struct {
	Common
	BinSuffix string `default:"netprobe.linux_64"`
	NetpHome  string `default:"{{join .Root \"netprobe\" \"netprobes\" .Name}}"`
	NetpBins  string `default:"{{join .Root \"packages\" \"netprobe\"}}"`
	NetpBase  string `default:"active_prod"`
	NetpExec  string `default:"{{join .NetpBins .NetpBase .BinSuffix}}"`
	NetpLogD  string `default:"{{.NetpHome}}"`
	NetpLogF  string `default:"netprobe.log"`
	NetpPort  int    `default:"7036"`
	NetpMode  string `default:"background"`
	NetpOpts  string // =-nopassword
	NetpLibs  string `default:"{{join .NetpBins .NetpBase \"lib64\"}}:{{join .NetpBins .NetpBase}}"`
	NetpUser  string
	NetpCert  string `default:"{{join .NetpHome \"netprobe.pem\"}}"`
	NetpKey   string `default:"{{join .NetpHome \"netprobe.key\"}}"`
}

const netprobePortRange = "7036,7100-"

func init() {
	components[Netprobe] = ComponentFuncs{
		Instance: netprobeInstance,
		Command:  netprobeCommand,
		New:      netprobeNew,
		Clean:    netprobeClean,
		Reload:   netprobeReload,
	}
}

func netprobeInstance(name string) interface{} {
	// Bootstrap
	c := &Netprobes{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Netprobe.String()
	c.Name = name
	setDefaults(&c)
	return c
}

func netprobeCommand(c Instance) (args, env []string) {
	certfile := getString(c, Prefix(c)+"Cert")
	keyfile := getString(c, Prefix(c)+"Key")
	logFile := getLogfilePath(c)
	args = []string{
		Name(c),
		"-port",
		getIntAsString(c, Prefix(c)+"Port"),
	}
	env = append(env, "LOG_FILENAME="+logFile)

	if certfile != "" {
		args = append(args, "-secure", "-ssl-certificate", certfile)
	}

	if keyfile != "" {
		args = append(args, "-ssl-certificate-key", keyfile)
	}

	return
}

// create a plain netprobe instance
func netprobeNew(name string, username string) (c Instance, err error) {
	// fill in the blanks
	c = netprobeInstance(name)
	if err = setField(c, Prefix(c)+"Port", strconv.Itoa(nextPort(RunningConfig.NetprobePortRange))); err != nil {
		return
	}
	if err = setField(c, Prefix(c)+"User", username); err != nil {
		return
	}
	conffile := filepath.Join(Home(c), Type(c).String()+".json")
	writeConfigFile(conffile, c)
	// default config XML etc.
	return
}

var defaultNetprobeCleanList = "*.old"
var defaultNetprobePurgeList = "netprobe.log:netprobe.txt:*.snooze:*.user_assignment"

func netprobeClean(c Instance, purge bool, params []string) (err error) {
	logDebug.Println(Type(c), Name(c), "clean")
	if purge {
		var stopped bool = true
		err = stopInstance(c, params)
		if err != nil {
			if errors.Is(err, ErrProcNotExist) {
				stopped = false
			} else {
				return err
			}
		}
		if err = removePathList(c, RunningConfig.NetprobeCleanList); err != nil {
			return err
		}
		err = removePathList(c, RunningConfig.NetprobePurgeList)
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return removePathList(c, RunningConfig.NetprobeCleanList)
}

func netprobeReload(c Instance, params []string) (err error) {
	return ErrNotSupported
}
