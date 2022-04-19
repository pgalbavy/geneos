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

func WriteConfig(c *Host) (err error) {
	file := ConfigPathWithExt(c, "json")
	return c.V().WriteConfigAs(file)
}

func ReadConfig(c *Host) (err error) {
	file := ConfigPathWithExt(c, "json")
	c.V().SetConfigFile(file)
	return c.V().MergeInConfig()
}

// return the full path to the host configuration file with the
// extension given
func ConfigPathWithExt(c *Host, extension string) (path string) {
	return filepath.Join(c.dir, "remote."+extension)
}
