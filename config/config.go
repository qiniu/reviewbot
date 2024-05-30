package config

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

type Config struct {
	GlobalDefaultConfig GlobalConfig `json:"globalDefaultConfig,omitempty"`

	// CustomConfig is the custom linter config.
	// e.g.
	// * "org/repo": {"golangci-lint": {"enable": true, "workDir": "", "command": "golangci-lint", "args": ["run", "--config", ".golangci.yml"], "reportFormat": "github_checks"}}
	// * "org": {"golangci-lint": {"enable": true, "workDir": "", "command": "golangci-lint", "args": ["run", "--config", ".golangci.yml"], "reportFormat": "github_checks"}}
	CustomConfig map[string]map[string]Linter `json:"customConfig,omitempty"`
}

type GlobalConfig struct {
	// GithubReportType is the format of the report, will be used if linterConfig.ReportFormat is empty.
	// e.g. "github_checks", "github_pr_review"
	GithubReportType GithubReportType `json:"githubReportType,omitempty"`
}

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

	// ReportFormat is the format of the report, if empty, use globalDefaultConfig.
	// For more details, see:
	// github_check_run: https://developer.github.com/v3/checks/runs/#create-a-check-run
	// github_pr_review: https://developer.github.com/v3/pulls/reviews/#create-a-pull-request-review
	// Note:
	// * github_check_run only support on Github Apps, not support on Github OAuth Apps or authenticated users.
	ReportFormat GithubReportType `json:"githubReportType,omitempty"`
}

func (l Linter) String() string {
	return fmt.Sprintf("Linter{Enable: %v, WorkDir: %v, Command: %v, Args: %v, ReportFormat: %v}", *l.Enable, l.WorkDir, l.Command, l.Args, l.ReportFormat)
}

// NewConfig returns a new Config.
func NewConfig(conf string) (Config, error) {
	var c Config
	f, err := os.ReadFile(conf)
	if err != nil {
		return c, err
	}

	if err = yaml.Unmarshal(f, &c); err != nil {
		return c, err
	}

	// set default value
	if c.GlobalDefaultConfig.GithubReportType == "" {
		c.GlobalDefaultConfig.GithubReportType = GithubPRReview
	}

	return c, nil
}

func (c Config) Get(org, repo, ln string) Linter {
	linter := Linter{
		Enable:       boolPtr(true),
		ReportFormat: c.GlobalDefaultConfig.GithubReportType,
		Command:      ln,
	}

	if orgConfig, ok := c.CustomConfig[org]; ok {
		if l, ok := orgConfig[ln]; ok {
			linter = applyCustomConfig(linter, l)
		}
	}

	if repoConfig, ok := c.CustomConfig[org+"/"+repo]; ok {
		if l, ok := repoConfig[ln]; ok {
			linter = applyCustomConfig(linter, l)
		}
	}

	return linter
}

func applyCustomConfig(legacy, custom Linter) Linter {
	if custom.Enable != nil {
		legacy.Enable = custom.Enable
	}

	if custom.WorkDir != "" {
		legacy.WorkDir = custom.WorkDir
	}

	if custom.Command != "" {
		legacy.Command = custom.Command
	}

	if len(custom.Args) != 0 {
		legacy.Args = custom.Args
	}

	if custom.ReportFormat != "" {
		legacy.ReportFormat = custom.ReportFormat
	}

	return legacy
}

// GithubReportType is the type of the report.
type GithubReportType string

const (
	GithubCheckRuns GithubReportType = "github_check_run"
	GithubPRReview  GithubReportType = "github_pr_review"
)

func boolPtr(b bool) *bool {
	return &b
}
