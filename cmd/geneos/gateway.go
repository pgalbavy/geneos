package main

import (
	"crypto/rand"
	"crypto/sha1"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/pbkdf2"
)

const Gateway Component = "gateway"

type Gateways struct {
	InstanceBase
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

	// The Gateway configuration name may be diffrent to the instance name
	GateName string `default:"{{.InstanceName}}"`

	// include files for gateway template - format is priority:path
	Includes map[int]string
}

//go:embed templates/gateway.setup.xml.gotmpl
var GatewayTemplate []byte

const GatewayDefaultTemplate = "gateway.setup.xml.gotmpl"

func init() {
	RegisterComponent(Components{
		Initialise:       InitGateway,
		New:              NewGateway,
		ComponentType:    Gateway,
		ParentType:       None,
		ComponentMatches: []string{"gateway", "gateways"},
		RealComponent:    true,
		DownloadBase:     "Gateway+2",
	})
	RegisterDirs([]string{
		"packages/gateway",
		"gateway/gateways",
		"gateway/gateway_shared",
		"gateway/gateway_config",
		"gateway/templates",
	})
	RegisterSettings(GlobalSettings{
		"GatewayPortRange": "7039,7100-",
		"GatewayCleanList": "*.old:*.history",
		"GatewayPurgeList": "gateway.log:gateway.txt:gateway.snooze:gateway.user_assignment:licences.cache:cache/:database/",
	})
}

func InitGateway(r *Remotes) {
	// copy default template to directory
	if err := r.writeFile(r.GeneosPath(Gateway.String(), "templates", GatewayDefaultTemplate), GatewayTemplate, 0664); err != nil {
		log.Fatalln(err)
	}
}

func NewGateway(name string) Instances {
	local, r := SplitInstanceName(name, rLOCAL)
	c := &Gateways{}
	c.InstanceRemote = r
	c.RemoteRoot = r.GeneosRoot()
	c.InstanceType = Gateway.String()
	c.InstanceName = local
	setDefaults(&c)
	c.InstanceLocation = RemoteName(r.InstanceName)
	return c
}

// interface method set

// Return the Component for an Instance
func (g Gateways) Type() Component {
	return parseComponentName(g.InstanceType)
}

func (g Gateways) Name() string {
	return g.InstanceName
}

func (g Gateways) Location() RemoteName {
	return g.InstanceLocation
}

func (g Gateways) Home() string {
	return g.GateHome
}

func (g Gateways) Prefix(field string) string {
	return "Gate" + field
}

func (g Gateways) Remote() *Remotes {
	return g.InstanceRemote
}

func (g Gateways) String() string {
	return g.Type().String() + ":" + g.InstanceName + "@" + g.Location().String()
}

func (g Gateways) Load() (err error) {
	if g.ConfigLoaded {
		return
	}
	err = loadConfig(g)
	g.ConfigLoaded = err == nil
	return
}

func (g Gateways) Unload() (err error) {
	g.ConfigLoaded = false
	return
}

func (g Gateways) Loaded() bool {
	return g.ConfigLoaded
}

func (g Gateways) Add(username string, params []string, tmpl string) (err error) {
	g.GatePort = g.InstanceRemote.nextPort(GlobalConfig["GatewayPortRange"])
	g.GateUser = username
	g.ConfigRebuild = "initial"
	g.Includes = make(map[int]string)

	if err = writeInstanceConfig(g); err != nil {
		logError.Fatalln(err)
	}
	// apply any extra args to settings
	if len(params) > 0 {
		commandSet(Gateway, []string{g.Name()}, params)
		loadConfig(&g)
	}

	// check tls config, create certs if found
	if _, err = readSigningCert(); err == nil {
		createInstanceCert(&g)
	}

	createAESKeyFile(&g)
	return g.Rebuild(true)
}

func (g Gateways) Rebuild(initial bool) error {
	if g.ConfigRebuild == "never" {
		return ErrNoAction
	}

	if !(g.ConfigRebuild == "always" || (initial && g.ConfigRebuild == "initial")) {
		return ErrNoAction
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

	if secure && g.GatePort == 7039 {
		g.GatePort = 7038
		changed = true
	} else if !secure && g.GatePort == 7038 {
		g.GatePort = 7039
		changed = true
	}

	if changed {
		if err := writeInstanceConfig(g); err != nil {
			log.Fatalln(err)
		}
	}
	return createConfigFromTemplate(g, filepath.Join(g.Home(), "gateway.setup.xml"), GatewayDefaultTemplate, GatewayTemplate)
}

func (c Gateways) Command() (args, env []string) {
	// get opts from
	// from https://docs.itrsgroup.com/docs/geneos/5.10.0/Gateway_Reference_Guide/gateway_installation_guide.html#Gateway_command_line_options
	//
	args = []string{
		c.Name(),
		"-resources-dir",
		filepath.Join(c.GateBins, c.GateBase, "resources"),
		"-log",
		getLogfilePath(c),
		// enable stats by default
		"-stats",
	}

	// check version
	// "-gateway-name",

	if c.GateName != c.Name() {
		args = append([]string{c.GateName}, args...)
	}

	args = append([]string{"-port", fmt.Sprint(c.GatePort)}, args...)

	if c.GateLicH != "" {
		args = append(args, "-licd-host", c.GateLicH)
	}

	if c.GateLicP != 0 {
		args = append(args, "-licd-port", fmt.Sprint(c.GateLicP))
	}

	if c.GateCert != "" {
		if c.GateLicS == "" || c.GateLicS != "false" {
			args = append(args, "-licd-secure")
		}
		args = append(args, "-ssl-certificate", c.GateCert)
		chainfile := c.Remote().GeneosPath("tls", "chain.pem")
		args = append(args, "-ssl-certificate-chain", chainfile)
	} else if c.GateLicS != "" && c.GateLicS == "true" {
		args = append(args, "-licd-secure")
	}

	if c.GateKey != "" {
		args = append(args, "-ssl-certificate-key", c.GateKey)
	}

	return
}

func (c Gateways) Clean(purge bool, params []string) (err error) {
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
		if err = deletePaths(c, GlobalConfig["GatewayCleanList"]); err != nil {
			return err
		}
		err = deletePaths(c, GlobalConfig["GatewayPurgeList"])
		if stopped {
			err = startInstance(c, params)
		}
		log.Println(c, "cleaned fully")
		return
	}
	err = deletePaths(c, GlobalConfig["GatewayCleanList"])
	if err == nil {
		log.Println(c, "cleaned")
	}
	return
}

func (c Gateways) Reload(params []string) (err error) {
	return signalInstance(c, syscall.SIGUSR1)
}

// create a gateway key file for secure passwrods as per
// https://docs.itrsgroup.com/docs/geneos/4.8.0/Gateway_Reference_Guide/gateway_secure_passwords.htm
func createAESKeyFile(c Instances) {
	rp := make([]byte, 20)
	// iv := make([]byte, 10)
	salt := make([]byte, 10)
	_, err := rand.Read(rp)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = rand.Read(salt)
	if err != nil {
		log.Fatalln(err)
	}
	md := pbkdf2.Key(rp, salt, 10000, 48, sha1.New)
	key := md[:32]
	iv := md[32:]
	keyfile := fmt.Sprintf("salt=%X\nkey=%X\niv =%X\n", salt, key, iv)
	err = c.Remote().writeFile(InstanceFile(c, "aes"), []byte(keyfile), 0400)
	if err != nil {
		log.Fatalln(err)
	}

}
