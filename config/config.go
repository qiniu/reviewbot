package config

import (
	"os"

	"sigs.k8s.io/yaml"
)

// Config maps org or repo to LinterConfig
//type Config map[string]map[string]Linter

type Config map[string]LinterList

type LinterList struct {
	StaticCheck Linter `json:"staticcheck" yaml:"staticcheck" default:"{\"enable\":true}"`
	GoVet       Linter `json:"govet" yaml:"govet" default:"{\"enable\":true}"`
	LuaCheck    Linter `json:"luacheck" yaml:"luacheck" default:"{\"enable\":true}"`
}

type Linter struct {
	// Enable is whether to enable this linter, if false, linter still run but not report.
	Enable  bool     `json:"enable" yaml:"enable"`
	WorkDir string   `json:"workDir" yaml:"workDir"`
	Command string   `json:"command" yaml:"command"`
	Args    []string `json:"args" yaml:"args"`
}

// NewConfig returns a new Config.
func NewConfig(conf string) (Config, error) {
	var c Config
	f, err := os.ReadFile(conf)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(f, &c); err != nil {
		return nil, err
	}

	for k, v := range c {
		if err = SetDefaults(&v); err != nil {
			return nil, err
		}
		c[k] = v
	}

	return c, nil
}

func (c Config) CustomLinterConfigs(org, repo string) LinterList {
	if repoConfig, ok := c[org+"/"+repo]; ok {
		return repoConfig
	}

	if orgConfig, ok := c[org]; ok {
		return orgConfig
	}

	return LinterList{}
}
