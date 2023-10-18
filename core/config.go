package core

import (
	"io/ioutil"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Default  Default                `toml:"DEFAULT"`
	SHDapi   SHDapi                 `toml:"SHDAPI"`
	Clusters map[string]ClusterConf `toml:"cluster"`
}

type Default struct {
	CredsPath   string `toml:"creds_path"`
	StaticPath  string `toml:"static_path"`
	ModulesPath string `toml:"modules_path"`
}

type ClusterConf struct {
	Conf string `toml:"conf"`
	// Keyring string `toml:"keyring"`
	User string `toml:"user"`
	Type string `toml:"type"`
}

type SHDapi struct {
	Enable bool `toml:"enable"`
}

func NewConfig(path string) (Config, error) {
	var config Config
	confFile, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer confFile.Close()

	content, err := ioutil.ReadAll(confFile)
	if err != nil {
		return config, err
	}

	if _, err := toml.Decode(string(content), &config); err != nil {
		return config, err
	}

	return config, nil
}
