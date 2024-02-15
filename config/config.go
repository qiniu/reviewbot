package config

import (
	"os"

	"sigs.k8s.io/yaml"
)

// Config maps org or repo to LinterConfig
type Config map[string]map[string]Linter

type Linter struct {
	// Enable is whether to enable this linter, if false, linter still run but not report.
	Enable  *bool    `json:"enable,omitempty"`
	WorkDir string   `json:"workDir,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
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

	// TODO: 应该默认开启staticcheck？

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
