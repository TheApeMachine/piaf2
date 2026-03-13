package core

import (
	"embed"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed cfg/*
var Embedded embed.FS

var Cfg *Config

/*
Config holds AI persona prompts and other settings loaded from config.yml.
*/
type Config struct {
	AI struct {
		Provider struct {
			OpenAI struct {
				Model   string `yaml:"model"`
				BaseURL string `yaml:"baseURL"`
			} `yaml:"openai"`
		} `yaml:"provider"`
		Persona struct {
			Research struct {
				Manager string `yaml:"manager"`
			} `yaml:"research"`
		} `yaml:"persona"`
	} `yaml:"ai"`
}

/*
Load reads config from the user's config path if it exists, otherwise from embedded cfg.
*/
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	override := filepath.Join(home, ".piaf", "config.yml")
	if data, err := os.ReadFile(override); err == nil {
		config := &Config{}
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, err
		}

		return config, nil
	}

	return LoadEmbedded(Embedded, "cfg/config.yml")
}

/*
LoadEmbedded reads config from the embedded filesystem.
Name is the path within the FS, e.g. "cfg/config.yml" or "config.yml".
*/
func LoadEmbedded(embedded embed.FS, name string) (*Config, error) {
	data, err := embedded.ReadFile(name)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	Cfg = config
	return config, nil
}
