package main

import (
	"errors"
	"os"
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
	WebsLogD string `default:"logs"`
	WebsLogF string `default:"webserver.log"`
	WebsMode string `default:"background"`
	WebsPort int    `default:"8080"`
	WebsOpts string `json:",omitempty"`
	WebsLibs string `default:"{{join .WebsBins .WebsBase \"JRE/lib\"}}:{{join .WebsBins .WebsBase \"lib64\"}}"`
	WebsXmx  string `default:"1024M"`
	WebsUser string `json:",omitempty"`
	// certs have to be turned into java trust/key stores
	WebsCert string `json:",omitempty"`
	WebsKey  string `json:",omitempty"`
}

const webserverPortRange = "8080,8100-"

func init() {
	components[Webserver] = ComponentFuncs{
		Instance: webserverInstance,
		Command:  webserverCommand,
		Add:      webserverAdd,
		Clean:    webserverClean,
		Reload:   webserverReload,
	}
}

func webserverInstance(name string) interface{} {
	local, remote := splitInstanceName(name)
	c := &Webservers{}
	c.Root = remoteRoot(remote)
	c.Type = Webserver.String()
	c.Name = local
	c.Location = remote
	setDefaults(&c)
	return c
}

func webserverCommand(c Instance) (args, env []string) {
	WebsHome := getString(c, Prefix(c)+"Home")
	WebsBase := filepath.Join(getString(c, Prefix(c)+"Bins"), getString(c, Prefix(c)+"Base"))
	args = []string{
		// "-Duser.home=" + WebsHome,
		"-XX:+UseConcMarkSweepGC",
		"-Xmx" + getString(c, Prefix(c)+"Xmx"),
		"-server",
		"-Djava.io.tmpdir=" + WebsHome + "/webapps",
		"-Djava.awt.headless=true",
		"-DsecurityConfig=" + WebsHome + "/config/security.xml",
		"-Dcom.itrsgroup.configuration.file=" + WebsHome + "/config/config.xml",
		// "-Dcom.itrsgroup.dashboard.dir=<Path to dashboards directory>",
		"-Dcom.itrsgroup.dashboard.resources.dir=" + WebsBase + "/resources",
		"-Djava.library.path=" + getString(c, Prefix(c)+"Libs"),
		"-Dlog4j2.configurationFile=file:" + WebsHome + "/config/log4j2.properties",
		"-Dworking.directory=" + WebsHome,
		"-Dcom.itrsgroup.legacy.database.maxconnections=100",
		// SSO
		"-Dcom.itrsgroup.sso.config.file=" + WebsHome + "/config/sso.properties",
		"-Djava.security.auth.login.config=" + WebsHome + "/config/login.conf",
		"-Djava.security.krb5.conf=/etc/krb5.conf",
		"-Dcom.itrsgroup.bdosync=DataView,BDOSyncType_Level,DV1_SyncLevel_RedAmberCells",
		// "-Dcom.sun.management.jmxremote.port=$JMX_PORT -Dcom.sun.management.jmxremote.authenticate=false -Dcom.sun.management.jmxremote.ssl=false",
		"-XX:+HeapDumpOnOutOfMemoryError",
		"-XX:HeapDumpPath=/tmp",
		"-jar", WebsBase + "/geneos-web-server.jar",
		"-dir", WebsBase + "/webapps",
		"-port", getIntAsString(c, Prefix(c)+"Port"),
		// "-ssl true",
		"-maxThreads 254",
		// "-log", getLogfilePath(c),
	}

	return
}

// list of file patterns to copy?
// from WebBins + WebBase + /config

var webserverFiles = []string{
	"config/config.xml=config.xml.min.tmpl",
	"config/=log4j.properties",
	"config/=log4j2.properties",
	"config/=logging.properties",
	"config/=login.conf",
	"config/=security.properties",
	"config/=security.xml",
	"config/=sso.properties",
	"config/=users.properties",
}

func webserverAdd(name string, username string, params []string) (c Instance, err error) {
	// fill in the blanks
	c = webserverInstance(name)
	webport := strconv.Itoa(nextPort(RunningConfig.WebserverPortRange))
	if webport != "8080" {
		if err = setField(c, Prefix(c)+"Port", webport); err != nil {
			return
		}
	}
	if err = setField(c, Prefix(c)+"User", username); err != nil {
		return
	}

	writeInstanceConfig(c)

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(c)
	}

	// copy default configs - use existing upload routines?
	dir, err := os.Getwd()
	defer os.Chdir(dir)
	configSrc := filepath.Join(getString(c, Prefix(c)+"Bins"), getString(c, Prefix(c)+"Base"), "config")
	if err = os.Chdir(configSrc); err != nil {
		return
	}

	if err = mkdirAll(RemoteName(c), filepath.Join(Home(c), "webapps"), 0777); err != nil {
		return
	}

	for _, source := range webserverFiles {
		if err = uploadFile(c, source); err != nil {
			return
		}
	}

	return
}

var defaultWebserverCleanList = "*.old"
var defaultWebserverPurgeList = "webserver.log:webserver.txt"

func webserverClean(c Instance, purge bool, params []string) (err error) {
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
