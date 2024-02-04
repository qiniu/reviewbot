package config

import (
	"os"

	"sigs.k8s.io/yaml"
)

// Config maps org or repo to LinterConfig
//type Config map[string]map[string]Linter

type Config map[string]map[string]Linter

type Linter struct {
	// Enable is whether to enable this linter, if false, linter still run but not report.
	Enable  *bool    `json:"enable,omitempty" yaml:"enable,omitempty"`
	WorkDir string   `json:"workDir,omitempty" yaml:"workDir,omitempty"`
	Command string   `json:"command,omitempty" yaml:"command,omitempty"`
	Args    []string `json:"args,omitempty" yaml:"args,omitempty"`
}

// NewConfig returns a new Config.
func NewConfig(conf string) (Config, error) {
	f, err := os.ReadFile(conf)
	if err != nil {
		return nil, err
	}

	c := Config{}
	if err = yaml.Unmarshal(f, &c); err != nil {
		return nil, err
	}

	defaultEnable := true
	for _, v := range c {
		for _, val := range v {
			if val.Enable == nil {
				val.Enable = &defaultEnable
			}
		}
	}

	if len(c) == 0 {
		c["qbox"] = map[string]Linter{
			"staticcheck": {Enable: &defaultEnable},
			"govet":       {Enable: &defaultEnable},
			"luacheck":    {Enable: &defaultEnable},
		}
	}

	return c, nil
}

func (c Config) CustomLinterConfigs(org, repo string) map[string]Linter {
	if repoConfig, ok := c[org+"/"+repo]; ok {
		return repoConfig
	}

	if orgConfig, ok := c[org]; ok {
		return orgConfig
	}

	return nil
}
