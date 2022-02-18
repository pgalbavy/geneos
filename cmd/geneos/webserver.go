package main

import (
	"errors"
	"path/filepath"
	"strconv"
)

type Webservers struct {
	Common
	//BinSuffix string `default:"licd.linux_64"`
	WebsHome string `default:"{{join .Root \"webserver\" \"webservers\" .Name}}"`
	WebsBins string `default:"{{join .Root \"packages\" \"webserver\"}}"`
	WebsBase string `default:"active_prod"`
	WebsExec string `default:"{{join .WebsBins .WebsBase \"JRE/bin/java\"}}"`
	WebsLogD string
	WebsLogF string `default:"webserver.log"`
	WebsMode string `default:"background"`
	WebsPort int    `default:"8080"`
	WebsOpts string
	WebsLibs string `default:"{{join .WebsBins .WebsBase \"JRE/lib\"}}:{{join .WebsBins .WebsBase \"lib64\"}}"`
	WebsXmx  string `default:"512M"`

	WebsUser string
}

const webserverPortRange = "7041,7100-"

func init() {
	components[Webserver] = ComponentFuncs{
		Instance: webserverInstance,
		Command:  webserverCommand,
		New:      webserverNew,
		Clean:    webserverClean,
		Reload:   webserverReload,
	}
}

func webserverInstance(name string) interface{} {
	// Bootstrap
	c := &Webservers{}
	c.Root = RunningConfig.ITRSHome
	c.Type = Webserver.String()
	c.Name = name
	setDefaults(&c)
	return c
}

func webserverCommand(c Instance) (args, env []string) {
	WebsHome := getString(c, Prefix(c)+"Home")
	WebsBase := filepath.Join(getString(c, Prefix(c)+"Bins"), getString(c, Prefix(c)+"Base"))
	args = []string{
		"-Duser.home=" + WebsHome,
		"-Xmx" + getString(c, Prefix(c)+"Xmx"),
		"-Djava.awt.headless=true",
		"-Dcom.itrsgroup.configuration.file=" + WebsHome + "/config/config.xml",
		"-DsecurityConfig=" + WebsHome + "/config/security.xml",
		"-Dcom.itrsgroup.dashboard.resources.dir=" + WebsBase + "/resources",
		"-Djava.library.path=" + getString(c, Prefix(c)+"Libs"),
		"-Dlog4j.configuration=file:" + WebsHome + "/config/log4j.properties",
		"-Dworking.directory=" + WebsHome,
		"-Dcom.itrsgroup.legacy.database.maxconnections=100",
		"-Dcom.itrsgroup.bdosync=DataView,BDOSyncType_Level,DV1_SyncLevel_RedAmberCells",
		// "-Dcom.sun.management.jmxremote.port=$JMX_PORT -Dcom.sun.management.jmxremote.authenticate=false -Dcom.sun.management.jmxremote.ssl=false",
		"-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/tmp",
		"-jar", WebsBase + "/geneos-web-server.jar",
		"-dir", WebsBase + "/webapps",
		"-port", getIntAsString(c, Prefix(c)+"Port"),
		"-log", getLogfilePath(c),
	}

	return
}

func webserverNew(name string, username string) (c Instance, err error) {
	// fill in the blanks
	c = webserverInstance(name)
	if err = setField(c, Prefix(c)+"Port", strconv.Itoa(nextPort(RunningConfig.WebserverPortRange))); err != nil {
		return
	}
	if err = setField(c, Prefix(c)+"User", username); err != nil {
		return
	}
	conffile := filepath.Join(Home(c), Type(c).String()+".json")
	err = writeConfigFile(conffile, c)
	// default config XML etc.
	return
}

var defaultWebserverCleanList = "*.old"
var defaultWebserverPurgeList = "webserver.log:webserver.txt"

func webserverClean(c Instance, params []string) (err error) {
	logDebug.Println(Type(c), Name(c), "clean")
	if cleanForce {
		var stopped bool = true
		err = stopInstance(c, params)
		if err != nil {
			if errors.Is(err, ErrProcNotExist) {
				stopped = false
			} else {
				return err
			}
		}
		if err = removePathList(c, RunningConfig.WebserverCleanList); err != nil {
			return err
		}
		err = removePathList(c, RunningConfig.WebserverPurgeList)
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return removePathList(c, RunningConfig.WebserverCleanList)
}

func webserverReload(c Instance, params []string) (err error) {
	return ErrNotSupported
}
