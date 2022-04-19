package gateway

import (
	"crypto/rand"
	"crypto/sha1"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/viper"
	"golang.org/x/crypto/pbkdf2"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
	"wonderland.org/geneos/pkg/logger"
)

var Gateway geneos.Component = geneos.Component{
	Initialise:       InitGateway,
	Name:             "gateway",
	RelatedTypes:     nil,
	ComponentMatches: []string{"gateway", "gateways"},
	RealComponent:    true,
	DownloadBase:     "Gateway+2",
	PortRange:        "GatewayPortRange",
	CleanList:        "GatewayCleanList",
	PurgeList:        "GatewayPurgeList",
	DefaultSettings: map[string]string{
		"GatewayPortRange": "7039,7100-",
		"GatewayCleanList": "*.old:*.history",
		"GatewayPurgeList": "gateway.log:gateway.txt:gateway.snooze:gateway.user_assignment:licences.cache:cache/:database/",
	},
}

type Gateways struct {
	instance.Instance
	BinSuffix string `default:"gateway2.linux_64"`
	GateHome  string `default:"{{join .RemoteRoot \"gateway\" \"gateways\" .InstanceName}}"`
	GateBins  string `default:"{{join .RemoteRoot \"packages\" \"gateway\"}}"`
	GateBase  string `default:"active_prod"`
	GateExec  string `default:"{{join .GateBins .GateBase .BinSuffix}}"`
	GateLogD  string `json:",omitempty"`
	GateLogF  string `default:"gateway.log"`
	GatePort  int    `default:"7039" json:",omitempty"`
	GateMode  string `json:",omitempty"`
	GateLicP  int    `json:",omitempty"`
	GateLicH  string `json:",omitempty"`
	GateLicS  string `json:",omitempty"`
	GateOpts  string `json:",omitempty"`
	GateLibs  string `default:"{{join .GateBins .GateBase \"lib64\"}}:/usr/lib64"`
	GateUser  string `json:",omitempty"`
	GateCert  string `json:",omitempty"`
	GateKey   string `json:",omitempty"`
	GateAES   string `json:",omitempty"`

	// The Gateway configuration name may be diffrent to the instance name
	GateName string `default:"{{.InstanceName}}"`

	// include files for gateway template - format is priority:path
	Includes map[int]string
}

//go:embed templates/gateway.setup.xml.gotmpl
var GatewayTemplate []byte

//go:embed templates/gateway-instance.setup.xml.gotmpl
var InstanceTemplate []byte

const GatewayDefaultTemplate = "gateway.setup.xml.gotmpl"
const GatewayInstanceTemplate = "gateway-instance.setup.xml.gotmpl"

func init() {
	geneos.RegisterComponent(&Gateway, New)
	Gateway.RegisterDirs([]string{
		"packages/gateway",
		"gateway/gateways",
		"gateway/gateway_shared",
		"gateway/gateway_config",
		"gateway/templates",
	})
}

func InitGateway(r *host.Host, ct *geneos.Component) {
	// copy default template to directory
	if err := geneos.MakeComponentDirs(r, ct); err != nil {
		logger.Error.Fatalln(err)
	}
	if err := r.WriteFile(r.GeneosPath("gateway", "templates", GatewayDefaultTemplate), GatewayTemplate, 0664); err != nil {
		logger.Error.Fatalln(err)
	}
	if err := r.WriteFile(r.GeneosPath("gateway", "templates", GatewayInstanceTemplate), InstanceTemplate, 0664); err != nil {
		logger.Error.Fatalln(err)
	}
}

var gateways sync.Map

func New(name string) geneos.Instance {
	_, local, r := instance.SplitName(name, host.LOCAL)
	g, ok := gateways.Load(r.FullName(local))
	if ok {
		gw, ok := g.(*Gateways)
		if ok {
			return gw
		}
	}
	c := &Gateways{}
	c.Conf = viper.New()
	c.InstanceRemote = r
	c.RemoteRoot = r.GeneosRoot()
	c.Component = &Gateway
	c.InstanceName = local
	if err := instance.SetDefaults(c); err != nil {
		logger.Error.Fatalln(c, "setDefaults():", err)
	}
	c.InstanceHost = host.Name(r.String())
	gateways.Store(r.FullName(local), c)
	return c
}

// interface method set

// Return the Component for an Instance
func (g *Gateways) Type() *geneos.Component {
	return g.Component
}

func (g *Gateways) Name() string {
	return g.InstanceName
}

func (g *Gateways) Location() host.Name {
	return g.InstanceHost
}

func (g *Gateways) Home() string {
	return g.V().GetString("gatehome")
}

func (g *Gateways) Prefix(field string) string {
	return strings.ToLower("Gate" + field)
}

func (g *Gateways) Remote() *host.Host {
	return g.InstanceRemote
}

func (g *Gateways) Base() *instance.Instance {
	return &g.Instance
}

func (g *Gateways) String() string {
	return g.Type().String() + ":" + g.InstanceName + "@" + g.Location().String()
}

func (g *Gateways) Load() (err error) {
	if g.ConfigLoaded {
		return
	}
	logger.Debug.Printf("%v", g.V().AllSettings())
	// err = instance.LoadConfig(g)
	err = instance.ReadConfig(g)
	if err != nil {
		logger.Error.Println(err)
	}
	g.ConfigLoaded = err == nil
	return
}

func (g *Gateways) Unload() (err error) {
	gateways.Delete(g.Name() + "@" + g.Location().String())
	g.ConfigLoaded = false
	return
}

func (g *Gateways) Loaded() bool {
	return g.ConfigLoaded
}

func (g *Gateways) V() *viper.Viper {
	return g.Conf
}

func (g *Gateways) Add(username string, params []string, tmpl string) (err error) {
	g.GatePort = instance.NextPort(g.InstanceRemote, &Gateway)
	g.GateUser = username
	g.ConfigRebuild = "initial"
	g.Includes = make(map[int]string)

	// try to save config early
	if err = instance.WriteConfig(g); err != nil {
		return
	}

	// apply any extra args to settings
	// if len(params) > 0 {
	// 	if err = commandSet(Gateway, []string{g.Name()}, params); err != nil {
	// 		return
	// 	}
	// 	g.Load()
	// }

	// check tls config, create certs if found
	if _, err = instance.ReadSigningCert(); err == nil {
		if err = instance.CreateCert(g); err != nil {
			return
		}
	}

	if err = createAESKeyFile(g); err != nil {
		return
	}

	return g.Rebuild(true)
}

func (g *Gateways) Rebuild(initial bool) (err error) {
	err = instance.CreateConfigFromTemplate(g, filepath.Join(g.Home(), "instance.setup.xml"), GatewayInstanceTemplate, InstanceTemplate)
	if err != nil {
		return
	}

	if g.ConfigRebuild == "never" {
		return
	}

	if !(g.ConfigRebuild == "always" || (initial && g.ConfigRebuild == "initial")) {
		return
	}

	// recheck check certs/keys
	var changed bool
	secure := g.GateCert != "" && g.GateKey != ""

	// if we have certs then connect to Licd securely
	if secure && g.GateLicS != "true" {
		g.GateLicS = "true"
		changed = true
	} else if !secure && g.GateLicS == "true" {
		g.GateLicS = "false"
		changed = true
	}

	// use getPorts() to check valid change, else go up one
	ports := instance.GetPorts(g.Remote())
	nextport := instance.NextPort(g.Remote(), &Gateway)
	if secure && g.GatePort == 7039 {
		if _, ok := ports[7038]; !ok {
			g.GatePort = 7038
		} else {
			g.GatePort = nextport
		}
		changed = true
	} else if !secure && g.GatePort == 7038 {
		if _, ok := ports[7039]; !ok {
			g.GatePort = 7039
		} else {
			g.GatePort = nextport
		}
		changed = true
	}

	if changed {
		if err = instance.WriteConfig(g); err != nil {
			return
		}
	}

	return instance.CreateConfigFromTemplate(g, filepath.Join(g.Home(), "gateway.setup.xml"), GatewayDefaultTemplate, GatewayTemplate)
}

func (g *Gateways) Command() (args, env []string) {
	// get opts from
	// from https://docs.itrsgroup.com/docs/geneos/5.10.0/Gateway_Reference_Guide/gateway_installation_guide.html#Gateway_command_line_options
	//
	args = []string{
		g.Name(),
		"-resources-dir",
		filepath.Join(g.GateBins, g.GateBase, "resources"),
		"-log",
		instance.LogFile(g),
		"-setup",
		filepath.Join(g.GateHome, "gateway.setup.xml"),
		// enable stats by default
		"-stats",
	}

	// check version
	// "-gateway-name",

	if g.GateName != g.Name() {
		args = append([]string{g.GateName}, args...)
	}

	args = append([]string{"-port", fmt.Sprint(g.GatePort)}, args...)

	if g.GateLicH != "" {
		args = append(args, "-licd-host", g.GateLicH)
	}

	if g.GateLicP != 0 {
		args = append(args, "-licd-port", fmt.Sprint(g.GateLicP))
	}

	if g.GateCert != "" {
		if g.GateLicS == "" || g.GateLicS != "false" {
			args = append(args, "-licd-secure")
		}
		args = append(args, "-ssl-certificate", g.GateCert)
		chainfile := g.Remote().GeneosPath("tls", "chain.pem")
		args = append(args, "-ssl-certificate-chain", chainfile)
	} else if g.GateLicS != "" && g.GateLicS == "true" {
		args = append(args, "-licd-secure")
	}

	if g.GateKey != "" {
		args = append(args, "-ssl-certificate-key", g.GateKey)
	}

	// if c.GateAES != "" {
	// 	args = append(args, "-key-file", c.GateAES)
	// }

	return
}

func (g *Gateways) Reload(params []string) (err error) {
	return g.Signal(syscall.SIGUSR1)
}

func (g *Gateways) Signal(s syscall.Signal) error {
	return instance.Signal(g.Instance, s)
}

// create a gateway key file for secure passwords as per
// https://docs.itrsgroup.com/docs/geneos/4.8.0/Gateway_Reference_Guide/gateway_secure_passwords.htm
func createAESKeyFile(c geneos.Instance) (err error) {
	rp := make([]byte, 20)
	salt := make([]byte, 10)
	if _, err = rand.Read(rp); err != nil {
		return
	}
	if _, err = rand.Read(salt); err != nil {
		return
	}

	md := pbkdf2.Key(rp, salt, 10000, 48, sha1.New)
	key := md[:32]
	iv := md[32:]

	if err = c.Remote().WriteFile(instance.ConfigPathWithExt(c, "aes"), []byte(fmt.Sprintf("salt=%X\nkey=%X\niv =%X\n", salt, key, iv)), 0600); err != nil {
		return
	}
	c.V().Set(c.Prefix("AES"), c.Type().String()+".aes")
	return
}
