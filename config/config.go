package config

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

// Config is the config for the using of reviewbot.
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

	// GolangciLintConfig is the path of golangci-lint config file to run golangci-lint globally.
	// if not empty, use the config to run golangci-lint.
	// it can be overridden by linter.ConfigPath.
	GolangCiLintConfig string `json:"golangciLintConfig,omitempty"`
	// JavaPmdCheckRuleConfig is the path of javapmdcheck-lint rules config file to run javapmdcheck-lint globally.
	// if not empty, use the config to run javapmdcheck-lint.
	// it can be overridden by linter.ConfigPath.
	JavaPmdCheckRuleConfig string `json:"javapmdcheckruleConfig,omitempty"`
	// JavaStyleCheckRuleConfig is the path of javastylecheck-lint rules config file to run javastylecheck-lint globally.
	// if not empty, use the config to run javastylecheck-lint.
	// it can be overridden by linter.ConfigPath.
	JavaStyleCheckRuleConfig string `json:"javastylecheckruleConfig,omitempty"`
}

type DockerAsRunner struct {
	Image                string `json:"image"`
	CopyLinterFromOrigin bool   `json:"copylinterFromOrigin,omitempty"`
}
type Linter struct {
	// Name is the linter name.
	Name string
	// Enable is whether to enable this linter, if false, linter still run but not report.
	Enable *bool `json:"enable,omitempty"`
	// DockerAsRunner is the docker image to run the linter.
	// Optional, if not empty, use the docker image to run the linter.
	// e.g. "golang:1.23.4"
	DockerAsRunner DockerAsRunner `json:"dockerAsRunner,omitempty"`
	// WorkDir is the working directory of the linter.
	WorkDir string `json:"workDir,omitempty"`
	// Command is the command to run the linter. e.g. "golangci-lint", "staticcheck"
	// If empty, use the linter name as the command.
	Command []string `json:"command,omitempty"`
	// Args is the arguments of the command.
	Args []string `json:"args,omitempty"`
	// Env is the environment variables required for the linter execution.
	Env []string `json:"env,omitempty"`

	// ReportFormat is the format of the report, if empty, use globalDefaultConfig.
	// For more details, see:
	// github_check_run: https://developer.github.com/v3/checks/runs/#create-a-check-run
	// github_pr_review: https://developer.github.com/v3/pulls/reviews/#create-a-pull-request-review
	// Note:
	// * github_check_run only support on Github Apps, not support on Github OAuth Apps or authenticated users.
	ReportFormat GithubReportType `json:"githubReportType,omitempty"`

	// ConfigPath is the path of the linter config file.
	// If not empty, use the config to run the linter.
	ConfigPath string `json:"configPath,omitempty"`

	// Modifier knowns how to modify the linter command.
	Modifier Modifier
}

func (l Linter) String() string {
	return fmt.Sprintf(
		"Linter{Enable: %v, DockerAsRunner: %v, WorkDir: %v, Command: %v, Args: %v, ReportFormat: %v, ConfigPath: %v}",
		*l.Enable, l.DockerAsRunner, l.WorkDir, l.Command, l.Args, l.ReportFormat, l.ConfigPath)
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

	// check golangci-lint config path
	absPath, err := os.Getwd()
	if err != nil {
		return c, err
	}
	if c.GlobalDefaultConfig.GolangCiLintConfig != "" {
		c.GlobalDefaultConfig.GolangCiLintConfig = filepath.Join(absPath, c.GlobalDefaultConfig.GolangCiLintConfig)
		if _, err := os.Stat(c.GlobalDefaultConfig.GolangCiLintConfig); err != nil {
			return c, fmt.Errorf("golangci-lint config file not found: %v", c.GlobalDefaultConfig.GolangCiLintConfig)
		}
	}
	if c.GlobalDefaultConfig.JavaPmdCheckRuleConfig != "" {
		c.GlobalDefaultConfig.JavaPmdCheckRuleConfig = filepath.Join(absPath, c.GlobalDefaultConfig.JavaPmdCheckRuleConfig)
		if _, err := os.Stat(c.GlobalDefaultConfig.JavaPmdCheckRuleConfig); err != nil {
			return c, fmt.Errorf("java pmd check config file not found: %v", c.GlobalDefaultConfig.JavaPmdCheckRuleConfig)
		}
	}
	// check java style check config path
	if c.GlobalDefaultConfig.JavaStyleCheckRuleConfig != "" {
		c.GlobalDefaultConfig.JavaStyleCheckRuleConfig = filepath.Join(absPath, c.GlobalDefaultConfig.JavaStyleCheckRuleConfig)
		if _, err := os.Stat(c.GlobalDefaultConfig.JavaStyleCheckRuleConfig); err != nil {
			return c, fmt.Errorf("java style check config file not found: %v", c.GlobalDefaultConfig.JavaStyleCheckRuleConfig)
		}
	}

	return c, nil
}

func (c Config) Get(org, repo, ln string) Linter {
	linter := Linter{
		Enable:       boolPtr(true),
		ReportFormat: c.GlobalDefaultConfig.GithubReportType,
		Modifier:     NewBaseModifier(),
		Name:         ln,
	}

	// set golangci-lint config path if exists
	if c.GlobalDefaultConfig.GolangCiLintConfig != "" && ln == "golangci-lint" {
		linter.ConfigPath = c.GlobalDefaultConfig.GolangCiLintConfig
	}
	// check java pmd check config path
	if c.GlobalDefaultConfig.JavaPmdCheckRuleConfig != "" && ln == "pmdcheck" {
		linter.ConfigPath = c.GlobalDefaultConfig.JavaPmdCheckRuleConfig
	}
	// check java style check config path
	if c.GlobalDefaultConfig.JavaStyleCheckRuleConfig != "" && ln == "stylecheck" {
		linter.ConfigPath = c.GlobalDefaultConfig.JavaStyleCheckRuleConfig
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

	if linter.Command == nil {
		linter.Command = []string{ln}
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

	if custom.Command != nil {
		legacy.Command = custom.Command
	}

	if custom.Args != nil {
		legacy.Args = custom.Args
	}

	if custom.ReportFormat != "" {
		legacy.ReportFormat = custom.ReportFormat
	}

	if custom.ConfigPath != "" {
		legacy.ConfigPath = custom.ConfigPath
	}

	if custom.DockerAsRunner.Image != "" {
		legacy.DockerAsRunner.Image = custom.DockerAsRunner.Image
	}
	if custom.DockerAsRunner.CopyLinterFromOrigin {
		legacy.DockerAsRunner.CopyLinterFromOrigin = custom.DockerAsRunner.CopyLinterFromOrigin
	}

	return legacy
}

// GithubReportType is the type of the report.
type GithubReportType string

const (
	GithubCheckRuns GithubReportType = "github_check_run"
	GithubPRReview  GithubReportType = "github_pr_review"

	// for debug and testing.
	Quiet GithubReportType = "quiet"
)

func boolPtr(b bool) *bool {
	return &b
}

// Modifier defines the interface for modifying the linter command.
type Modifier interface {
	Modify(cfg *Linter) (*Linter, error)
}

type baseModifier struct{}

// NewBaseModifier returns a base modifier.
func NewBaseModifier() Modifier {
	return &baseModifier{}
}

func (*baseModifier) Modify(cfg *Linter) (*Linter, error) {
	// type: /bin/sh -c --
	if len(cfg.Command) > 0 {
		if cfg.Command[0] == "/bin/bash" || cfg.Command[0] == "/bin/sh" {
			return cfg, nil
		} else if cfg.Command[0] == "sh" {
			cfg.Command[0] = "/bin/sh"
			return cfg, nil
		} else if cfg.Command[0] == "bash" {
			cfg.Command[0] = "/bin/bash"
			return cfg, nil
		}
	}

	// TODO(CarlJi): other scenarios?

	newCfg := *cfg
	newCfg.Args = append(cfg.Command, cfg.Args...)
	newCfg.Command = []string{"/bin/sh", "-c", "--"}

	return &newCfg, nil
}

type contextKey string

// EventGUIDKey is the key for the event GUID in the context.
const EventGUIDKey contextKey = "event_guid"
