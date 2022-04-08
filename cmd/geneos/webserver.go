package main

import (
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
		RelatedTypes:     nil,
		ComponentMatches: []string{"web-server", "webserver", "webservers", "webdashboard", "dashboards"},
		RealComponent:    true,
		DownloadBase:     "Web+Dashboard",
		PortRange:        "WebserverPortRange",
		CleanList:        "WebserverCleanList",
		PurgeList:        "WebserverPurgeList",
	})
	Webserver.RegisterDirs([]string{
		"packages/webserver",
		"webserver/webservers",
	})
	RegisterDefaultSettings(GlobalSettings{
		"WebserverPortRange": "8080,8100-",
		"WebserverCleanList": "*.old",
		"WebserverPurgeList": "logs/*.log:webserver.txt",
	})
}

func NewWebserver(name string) Instances {
	_, local, r := SplitInstanceName(name, rLOCAL)
	c := &Webservers{}
	c.InstanceRemote = r
	c.RemoteRoot = r.GeneosRoot()
	c.InstanceType = Webserver.String()
	c.InstanceName = local
	if err := setDefaults(&c); err != nil {
		logError.Fatalln(c, "setDefauls():", err)
	}
	c.InstanceLocation = RemoteName(r.InstanceName)
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
func (w *Webservers) Type() Component {
	return parseComponentName(w.InstanceType)
}

func (w *Webservers) Name() string {
	return w.InstanceName
}

func (w *Webservers) Location() RemoteName {
	return w.InstanceLocation
}

func (w *Webservers) Home() string {
	return w.WebsHome
}

func (w *Webservers) Prefix(field string) string {
	return "Webs" + field
}

func (w *Webservers) Remote() *Remotes {
	return w.InstanceRemote
}

func (w *Webservers) Base() *InstanceBase {
	return &w.InstanceBase
}

func (w *Webservers) String() string {
	return w.Type().String() + ":" + w.InstanceName + "@" + w.Location().String()
}

func (w *Webservers) Load() (err error) {
	if w.ConfigLoaded {
		return
	}
	err = loadConfig(w)
	w.ConfigLoaded = err == nil
	return
}

func (w *Webservers) Unload() (err error) {
	w.ConfigLoaded = false
	return
}

func (w *Webservers) Loaded() bool {
	return w.ConfigLoaded
}

func (w *Webservers) Add(username string, params []string, tmpl string) (err error) {
	w.WebsPort = w.InstanceRemote.nextPort(Webserver)
	w.WebsUser = username

	if err = writeInstanceConfig(w); err != nil {
		return
	}

	// apply any extra args to settings
	if len(params) > 0 {
		if err = commandSet(Webserver, []string{w.Name()}, params); err != nil {
			return
		}
		w.Load()
	}

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		if err = createInstanceCert(w); err != nil {
			return
		}
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

func (w *Webservers) Rebuild(initial bool) error {
	return ErrNotSupported
}

func (w *Webservers) Command() (args, env []string) {
	WebsBase := filepath.Join(w.WebsBins, w.WebsBase)
	args = []string{
		// "-Duser.home=" + c.WebsHome,
		"-XX:+UseConcMarkSweepGC",
		"-Xmx" + w.WebsXmx,
		"-server",
		"-Djava.io.tmpdir=" + w.WebsHome + "/webapps",
		"-Djava.awt.headless=true",
		"-DsecurityConfig=" + w.WebsHome + "/config/security.xml",
		"-Dcom.itrsgroup.configuration.file=" + w.WebsHome + "/config/config.xml",
		// "-Dcom.itrsgroup.dashboard.dir=<Path to dashboards directory>",
		"-Dcom.itrsgroup.dashboard.resources.dir=" + WebsBase + "/resources",
		"-Djava.library.path=" + w.WebsLibs,
		"-Dlog4j2.configurationFile=file:" + w.WebsHome + "/config/log4j2.properties",
		"-Dworking.directory=" + w.WebsHome,
		"-Dcom.itrsgroup.legacy.database.maxconnections=100",
		// SSO
		"-Dcom.itrsgroup.sso.config.file=" + w.WebsHome + "/config/sso.properties",
		"-Djava.security.auth.login.config=" + w.WebsHome + "/config/login.conf",
		"-Djava.security.krb5.conf=/etc/krb5.conf",
		"-Dcom.itrsgroup.bdosync=DataView,BDOSyncType_Level,DV1_SyncLevel_RedAmberCells",
		// "-Dcom.sun.management.jmxremote.port=$JMX_PORT -Dcom.sun.management.jmxremote.authenticate=false -Dcom.sun.management.jmxremote.ssl=false",
		"-XX:+HeapDumpOnOutOfMemoryError",
		"-XX:HeapDumpPath=/tmp",
		"-jar", WebsBase + "/geneos-web-server.jar",
		"-dir", WebsBase + "/webapps",
		"-port", strconv.Itoa(w.WebsPort),
		// "-ssl true",
		"-maxThreads 254",
		// "-log", getLogfilePath(c),
	}

	return
}

func (w *Webservers) Reload(params []string) (err error) {
	return ErrNotSupported
}
