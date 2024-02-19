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

	// ReportFormat is the format of the report, if empty, use github_checks by default.
	// e.g. "github_checks", "github_pr_review"
	// For more details, see:
	// github_checks: https://developer.github.com/v3/checks/runs/#create-a-check-run
	// github_pr_review: https://developer.github.com/v3/pulls/reviews/#create-a-pull-request-review
	ReportFormat string `json:"reportFormat,omitempty"`
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

	c = fixConfig(c)

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

// fixConfig fix the config
// 1. if linterConfig.Enable is nil, set it to true
// 2. if linterConfig.ReportFormat is empty, set it to "github_checks"
func fixConfig(c Config) Config {
	for org, repoConfig := range c {
		for repo, linterConfig := range repoConfig {
			if linterConfig.Enable == nil {
				enable := true
				linterConfig.Enable = &enable
			}

			if linterConfig.ReportFormat == "" {
				linterConfig.ReportFormat = "github_checks"
			}

			c[org][repo] = linterConfig
		}
	}

	return c
}
