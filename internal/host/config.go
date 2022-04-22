package host

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func ReadLocalConfigFile(file string, config interface{}) (err error) {
	jsonFile, err := os.ReadFile(file)
	if err != nil {
		return
	}
	// dec := json.NewDecoder(jsonFile)
	return json.Unmarshal(jsonFile, &config)
}

func (h *Host) ReadConfigFile(file string, config interface{}) (jsonFile []byte, err error) {
	if jsonFile, err = h.ReadFile(file); err != nil {
		return
	}
	// dec := json.NewDecoder(jsonFile)
	return jsonFile, json.Unmarshal(jsonFile, &config)
}

// read a host configuration file. the host passed as an
// argument must already have been initialised with New()
func ReadConfig(c *Host) (err error) {
	file := ConfigFile(c, "json")
	c.V().SetConfigFile(file)
	return c.V().MergeInConfig()
}

// write out a host configuration file.
func WriteConfig(c *Host) (err error) {
	file := ConfigFile(c, "json")
	return c.V().WriteConfigAs(file)
}

// return the full path to the host configuration file with the
// extension given
func ConfigFile(c *Host, extension string) (path string) {
	return filepath.Join(c.Home, "remote."+extension)
}

func FindHostDirs() (hosts []string) {
	dir := filepath.Join(Geneos(), "remotes")
	dirs, err := LOCAL.ReadDir(dir)
	if err != nil {
		logError.Println(err)
	}
	for _, d := range dirs {
		if d.IsDir() {
			hosts = append(hosts, d.Name())
		}
	}
	return
}
