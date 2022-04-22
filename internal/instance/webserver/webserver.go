package webserver

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/pkg/logger"
)

var Webserver geneos.Component = geneos.Component{
	Name:             "webserver",
	RelatedTypes:     nil,
	ComponentMatches: []string{"web-server", "webserver", "webservers", "webdashboard", "dashboards"},
	RealComponent:    true,
	DownloadBase:     "Web+Dashboard",
	PortRange:        "WebserverPortRange",
	CleanList:        "WebserverCleanList",
	PurgeList:        "WebserverPurgeList",
	Defaults: []string{
		"webshome={{join .remoteroot \"webserver\" \"webservers\" .name}}",
		"websbins={{join .remoteroot \"packages\" \"webserver\"}}",
		"websbase=active_prod",
		"websexec={{join .websbins .websbase \"jre/bin/java\"}}",
		"webslogd=logs",
		"webslogf=webdashboard.log",
		"websmode=background",
		"websport=8080",
		"webslibs={{join .websbins .websbase \"jre/lib\"}}:{{join .websbins .websbase \"lib64\"}}",
		"websxmx =1024m",
	},
	GlobalSettings: map[string]string{
		"WebserverPortRange": "8080,8100-",
		"WebserverCleanList": "*.old",
		"WebserverPurgeList": "logs/*.log:webserver.txt",
	},
	Directories: []string{
		"packages/webserver",
		"webserver/webservers",
	},
}

type Webservers instance.Instance

func init() {
	geneos.RegisterComponent(&Webserver, New)
}

var webservers sync.Map

func New(name string) geneos.Instance {
	_, local, r := instance.SplitName(name, host.LOCAL)
	w, ok := webservers.Load(r.FullName(local))
	if ok {
		ws, ok := w.(*Webservers)
		if ok {
			return ws
		}
	}
	c := &Webservers{}
	c.Conf = viper.New()
	c.InstanceHost = r
	// c.RemoteRoot = r.V().GetString("geneos")
	c.Component = &Webserver
	if err := instance.SetDefaults(c, local); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	webservers.Store(r.FullName(local), c)
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
func (w *Webservers) Type() *geneos.Component {
	return w.Component
}

func (w *Webservers) Name() string {
	return w.V().GetString("name")
}

func (w *Webservers) Home() string {
	return w.V().GetString("webshome")
}

func (w *Webservers) Prefix(field string) string {
	return strings.ToLower("Webs" + field)
}

func (w *Webservers) Host() *host.Host {
	return w.InstanceHost
}

func (w *Webservers) String() string {
	return w.Type().String() + ":" + w.Name() + "@" + w.Host().String()
}

func (w *Webservers) Load() (err error) {
	if w.ConfigLoaded {
		return
	}
	err = instance.LoadConfig(w)
	w.ConfigLoaded = err == nil
	return
}

func (w *Webservers) Unload() (err error) {
	webservers.Delete(w.Name() + "@" + w.Host().String())
	w.ConfigLoaded = false
	return
}

func (w *Webservers) Loaded() bool {
	return w.ConfigLoaded
}

func (w *Webservers) V() *viper.Viper {
	return w.Conf
}

func (w *Webservers) Add(username string, params []string, tmpl string) (err error) {
	w.V().Set("websport", instance.NextPort(w.InstanceHost, &Webserver))
	w.V().Set("websuser", username)

	if err = instance.WriteConfig(w); err != nil {
		return
	}

	// apply any extra args to settings
	// if len(params) > 0 {
	// 	if err = commandSet(Webserver, []string{w.Name()}, params); err != nil {
	// 		return
	// 	}
	// 	w.Load()
	// }

	// check tls config, create certs if found
	if _, err = instance.ReadSigningCert(); err == nil {
		if err = instance.CreateCert(w); err != nil {
			return
		}
	}

	// copy default configs - use existing import routines?
	dir, err := os.Getwd()
	defer os.Chdir(dir)
	configSrc := filepath.Join(w.V().GetString("websbins"), w.V().GetString("websbase"), "config")
	if err = os.Chdir(configSrc); err != nil {
		return
	}

	if err = w.Host().MkdirAll(filepath.Join(w.Home(), "webapps"), 0775); err != nil {
		return
	}

	for _, source := range webserverFiles {
		if err = instance.ImportFile(w.Host(), w.Home(), w.V().GetString(w.Prefix("User")), source); err != nil {
			return
		}
	}

	return
}

func (w *Webservers) Rebuild(initial bool) error {
	return geneos.ErrNotSupported
}

func (w *Webservers) Command() (args, env []string) {
	WebsBase := filepath.Join(w.V().GetString("websbins"), w.V().GetString("websbase"))
	args = []string{
		// "-Duser.home=" + c.WebsHome,
		"-XX:+UseConcMarkSweepGC",
		"-Xmx" + w.V().GetString("websxmx"),
		"-server",
		"-Djava.io.tmpdir=" + w.V().GetString("webshome") + "/webapps",
		"-Djava.awt.headless=true",
		"-DsecurityConfig=" + w.V().GetString("webshome") + "/config/security.xml",
		"-Dcom.itrsgroup.configuration.file=" + w.V().GetString("webshome") + "/config/config.xml",
		// "-Dcom.itrsgroup.dashboard.dir=<Path to dashboards directory>",
		"-Dcom.itrsgroup.dashboard.resources.dir=" + WebsBase + "/resources",
		"-Djava.library.path=" + w.V().GetString("webslibs"),
		"-Dlog4j2.configurationFile=file:" + w.V().GetString("webshome") + "/config/log4j2.properties",
		"-Dworking.directory=" + w.V().GetString("webshome"),
		"-Dcom.itrsgroup.legacy.database.maxconnections=100",
		// SSO
		"-Dcom.itrsgroup.sso.config.file=" + w.V().GetString("webshome") + "/config/sso.properties",
		"-Djava.security.auth.login.config=" + w.V().GetString("webshome") + "/config/login.conf",
		"-Djava.security.krb5.conf=/etc/krb5.conf",
		"-Dcom.itrsgroup.bdosync=DataView,BDOSyncType_Level,DV1_SyncLevel_RedAmberCells",
		// "-Dcom.sun.management.jmxremote.port=$JMX_PORT -Dcom.sun.management.jmxremote.authenticate=false -Dcom.sun.management.jmxremote.ssl=false",
		"-XX:+HeapDumpOnOutOfMemoryError",
		"-XX:HeapDumpPath=/tmp",
		"-jar", WebsBase + "/geneos-web-server.jar",
		"-dir", WebsBase + "/webapps",
		"-port", w.V().GetString("websport"),
		// "-ssl true",
		"-maxThreads 254",
		// "-log", LogFile(c),
	}

	return
}

func (w *Webservers) Reload(params []string) (err error) {
	return geneos.ErrNotSupported
}
