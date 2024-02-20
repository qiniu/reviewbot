package config

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// Config maps org or repo to LinterConfig
type Config map[string]map[string]Linter

type Linter struct {
	// Enable is whether to enable this linter, if false, linter still run but not report.
	Enable *bool `json:"enable,omitempty"`
	// WorkDir is the working directory of the linter.
	WorkDir string `json:"workDir,omitempty"`
	// Command is the command to run the linter. e.g. "golangci-lint", "staticcheck"
	// If empty, use the linter name as the command.
	Command string `json:"command,omitempty"`
	// Args is the arguments of the command.
	Args []string `json:"args,omitempty"`

	// ReportFormat is the format of the report, if empty, use github_checks by default.
	// e.g. "github_checks", "github_pr_review"
	// For more details, see:
	// github_checks: https://developer.github.com/v3/checks/runs/#create-a-check-run
	// github_pr_review: https://developer.github.com/v3/pulls/reviews/#create-a-pull-request-review
	ReportFormat string `json:"reportFormat,omitempty"`
}

func (l Linter) String() string {
	return fmt.Sprintf("Linter{Enable: %v, WorkDir: %v, Command: %v, Args: %v, ReportFormat: %v}", *l.Enable, l.WorkDir, l.Command, l.Args, l.ReportFormat)
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

	c = FixConfig(c)

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

// FixConfig fix the config
// 1. if linterConfig.Enable is nil, set it to true
// 2. if linterConfig.ReportFormat is empty, set it to "github_checks"
// 3. if linterConfig.Command is empty, set it to linterName
func FixConfig(c Config) Config {
	for repo, repoConfig := range c {
		for linterName, linterConfig := range repoConfig {
			c[repo][linterName] = FixLinterConfig(linterConfig, linterName)
		}
	}

	return c
}

// FixLinterConfig fix the linter config
func FixLinterConfig(linterConfig Linter, linterName string) Linter {
	// if linterConfig.Enable is nil, set it to true
	if linterConfig.Enable == nil {
		enable := true
		linterConfig.Enable = &enable
	}

	// if linterConfig.ReportFormat is empty, set it to "github_checks"
	if linterConfig.ReportFormat == "" {
		linterConfig.ReportFormat = "github_checks"
	}

	// if linterConfig.Command is empty, set it to linterName
	if linterConfig.Command == "" {
		linterConfig.Command = linterName
	}

	return linterConfig
}
