package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

const Webserver Component = "webserver"

type Webservers struct {
	InstanceBase
	//BinSuffix string `default:"licd.linux_64"`
	WebsHome string `default:"{{join .RemoteRoot \"webserver\" \"webservers\" .InstanceName}}"`
	WebsBins string `default:"{{join .RemoteRoot \"packages\" \"webserver\"}}"`
	WebsBase string `default:"active_prod"`
	WebsExec string `default:"{{join .WebsBins .WebsBase \"JRE/bin/java\"}}"`
	WebsLogD string `default:"logs"`
	WebsLogF string `default:"WebDashboard.log"`
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

func init() {
	RegisterComponent(Components{
		New:              NewWebserver,
		ComponentType:    Webserver,
		ParentType:       None,
		ComponentMatches: []string{"web-server", "webserver", "webservers", "webdashboard", "dashboards"},
		RealComponent:    true,
		DownloadBase:     "Web+Dashboard",
	})
	RegisterDirs([]string{
		"packages/webserver",
		"webserver/webservers",
	})
	RegisterSettings(GlobalSettings{
		"WebserverPortRange": "8080,8100-",
		"WebserverCleanList": "*.old",
		"WebserverPurgeList": "webserver.log:webserver.txt",
	})
}

func NewWebserver(name string) Instances {
	local, remote := splitInstanceName(name)
	c := &Webservers{}
	c.InstanceRemote = GetRemote(remote)
	c.RemoteRoot = c.Remote().GeneosRoot()
	c.InstanceType = Webserver.String()
	c.InstanceName = local
	setDefaults(&c)
	c.InstanceLocation = remote
	return c
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

// interface method set

// Return the Component for an Instance
func (w Webservers) Type() Component {
	return parseComponentName(w.InstanceType)
}

func (w Webservers) Name() string {
	return w.InstanceName
}

func (w Webservers) Location() RemoteName {
	return w.InstanceLocation
}

func (w Webservers) Home() string {
	return w.WebsHome
}

func (w Webservers) Prefix(field string) string {
	return "Webs" + field
}

func (w Webservers) Remote() *Remotes {
	return w.InstanceRemote
}

func (w Webservers) String() string {
	return w.Type().String() + ":" + w.InstanceName + "@" + w.Location().String()
}

func (w Webservers) Add(username string, params []string, tmpl string) (err error) {
	w.WebsPort = w.InstanceRemote.nextPort(GlobalConfig["WebserverPortRange"])
	w.WebsUser = username

	if err = writeInstanceConfig(w); err != nil {
		logError.Fatalln(err)
	}

	// apply any extra args to settings
	if len(params) > 0 {
		commandSet(Webserver, []string{w.Name()}, params)
		loadConfig(&w)
	}

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(&w)
	}

	// copy default configs - use existing import routines?
	dir, err := os.Getwd()
	defer os.Chdir(dir)
	configSrc := filepath.Join(w.WebsBins, w.WebsBase, "config")
	if err = os.Chdir(configSrc); err != nil {
		return
	}

	if err = w.Remote().mkdirAll(filepath.Join(w.Home(), "webapps"), 0775); err != nil {
		return
	}

	for _, source := range webserverFiles {
		if err = importFile(w, source); err != nil {
			return
		}
	}

	return
}

func (w Webservers) Rebuild(initial bool) error {
	return ErrNotSupported
}

func (c Webservers) Command() (args, env []string) {
	WebsBase := filepath.Join(c.WebsBins, c.WebsBase)
	args = []string{
		// "-Duser.home=" + c.WebsHome,
		"-XX:+UseConcMarkSweepGC",
		"-Xmx" + c.WebsXmx,
		"-server",
		"-Djava.io.tmpdir=" + c.WebsHome + "/webapps",
		"-Djava.awt.headless=true",
		"-DsecurityConfig=" + c.WebsHome + "/config/security.xml",
		"-Dcom.itrsgroup.configuration.file=" + c.WebsHome + "/config/config.xml",
		// "-Dcom.itrsgroup.dashboard.dir=<Path to dashboards directory>",
		"-Dcom.itrsgroup.dashboard.resources.dir=" + WebsBase + "/resources",
		"-Djava.library.path=" + c.WebsLibs,
		"-Dlog4j2.configurationFile=file:" + c.WebsHome + "/config/log4j2.properties",
		"-Dworking.directory=" + c.WebsHome,
		"-Dcom.itrsgroup.legacy.database.maxconnections=100",
		// SSO
		"-Dcom.itrsgroup.sso.config.file=" + c.WebsHome + "/config/sso.properties",
		"-Djava.security.auth.login.config=" + c.WebsHome + "/config/login.conf",
		"-Djava.security.krb5.conf=/etc/krb5.conf",
		"-Dcom.itrsgroup.bdosync=DataView,BDOSyncType_Level,DV1_SyncLevel_RedAmberCells",
		// "-Dcom.sun.management.jmxremote.port=$JMX_PORT -Dcom.sun.management.jmxremote.authenticate=false -Dcom.sun.management.jmxremote.ssl=false",
		"-XX:+HeapDumpOnOutOfMemoryError",
		"-XX:HeapDumpPath=/tmp",
		"-jar", WebsBase + "/geneos-web-server.jar",
		"-dir", WebsBase + "/webapps",
		"-port", strconv.Itoa(c.WebsPort),
		// "-ssl true",
		"-maxThreads 254",
		// "-log", getLogfilePath(c),
	}

	return
}

func (c Webservers) Clean(purge bool, params []string) (err error) {
	logDebug.Println(c.Type(), c.Name(), "clean")
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
		if err = deletePaths(c, GlobalConfig["WebserverCleanList"]); err != nil {
			return err
		}
		err = deletePaths(c, GlobalConfig["WebserverPurgeList"])
		if stopped {
			err = startInstance(c, params)
		}
		return
	}
	return deletePaths(c, GlobalConfig["WebserverCleanList"])
}

func (c Webservers) Reload(params []string) (err error) {
	return ErrNotSupported
}
