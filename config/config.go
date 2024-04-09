package config

import (
	"fmt"
	"os"

	"github.com/qiniu/x/log"
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

	c = fixGlobalConfig(c)
	c = fixConfig(c)

	return c, nil
}

// CustomLinterConfigs returns the custom linter configs of the org or repo.
func (c Config) CustomLinterConfigs(org, repo string) map[string]Linter {
	if repoConfig, ok := c.CustomConfig[org+"/"+repo]; ok {
		return repoConfig
	}

	if orgConfig, ok := c.CustomConfig[org]; ok {
		return orgConfig
	}

	return nil
}

// FixLinterConfig fix the linter config if some fields are empty.
func (c Config) FixLinterConfig(linterConfig Linter, linterName string) Linter {
	// if linterConfig.Enable is nil, set it to true
	if linterConfig.Enable == nil {
		enable := true
		linterConfig.Enable = &enable
	}

	// if linterConfig.ReportFormat is empty, set it to "github_checks"
	if linterConfig.ReportFormat == "" {
		linterConfig.ReportFormat = c.GlobalDefaultConfig.GithubReportType
	}

	// if linterConfig.Command is empty, set it to linterName
	if linterConfig.Command == "" {
		linterConfig.Command = linterName
	}

	return linterConfig
}

// fixConfig fix the config
// 1. if linterConfig.Enable is nil, set it to true
// 2. if linterConfig.ReportFormat is empty, use globalDefaultConfig
// 3. if linterConfig.Command is empty, set it to linterName
func fixConfig(c Config) Config {
	for repo, repoConfig := range c.CustomConfig {
		for linterName, linterConfig := range repoConfig {
			c.CustomConfig[repo][linterName] = c.FixLinterConfig(linterConfig, linterName)
		}
	}

	return c
}

// fixGlobalConfig fix the global config
// 1. if globalDefaultConfig is empty, set it to "github_pr_review" by default
func fixGlobalConfig(c Config) Config {
	switch c.GlobalDefaultConfig.GithubReportType {
	case GithubCheckRuns:
		c.GlobalDefaultConfig.GithubReportType = GithubCheckRuns
	case GithubPRReview:
		c.GlobalDefaultConfig.GithubReportType = GithubPRReview
	default:
		log.Warnf("invalid globalDefaultConfig: %v, use %v by default", c.GlobalDefaultConfig.GithubReportType, GithubPRReview)
		c.GlobalDefaultConfig.GithubReportType = GithubPRReview
	}

	return c
}

// GithubReportType is the type of the report.
type GithubReportType string

const (
	GithubCheckRuns GithubReportType = "github_check_run"
	GithubPRReview  GithubReportType = "github_pr_review"
)
