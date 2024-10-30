package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/qiniu/x/log"
	"sigs.k8s.io/yaml"
)

// Config is the config for the using of reviewbot.
type Config struct {
	GlobalDefaultConfig GlobalConfig `json:"globalDefaultConfig,omitempty"`

	// CustomConfig is the custom org or repo config.
	// e.g.
	// * "org/repo": {"extraRefs":{org:xxx, repo:xxx, path_alias:github.com/repo }, "golangci-lint": {"enable": true, "workDir": "", "command": "golangci-lint", "args": ["run", "--config", ".golangci.yml"], "githubReportFormat": "github_checks"}}
	// * "org": {"extraRefs":{org:xxx, repo:xxx, path_alias:github.com/repo }, "golangci-lint": {"enable": true, "workDir": "", "command": "golangci-lint", "args": ["run", "--config", ".golangci.yml"], "githubReportFormat": "github_checks"}}
	CustomConfig map[string]RepoConfig `json:"customConfig,omitempty"`

	// IssueReferences is the issue references config.
	// key is the linter name.
	// value is the issue references config.
	IssueReferences map[string][]IssueReference `json:"issueReferences,omitempty"`
	// compiledIssueReferences is the compiled issue references config.
	compiledIssueReferences map[string][]CompiledIssueReference
}

type RepoConfig struct {
	// Refs are repositories that need to be cloned.
	// The main repository is cloned by default and does not need to be specified here if not specified.
	// extra refs must be specified.
	Refs    []Refs            `json:"refs,omitempty"`
	Linters map[string]Linter `json:"linters,omitempty"`
}

type Refs struct {
	// Org, Repo, and Host form a set that will ultimately be combined into a CloneURL
	// For example: Org="qiniu", Repo="kodo", Host="github.com"
	// will generate CloneURL="https://github.com/qiniu/kodo.git"
	//
	// Alternatively, you can directly provide the CloneURL, which will be parsed into the corresponding Org, Repo, and Host
	// Org, Repo, Host, and CloneURL are equivalent; only one set needs to be filled
	Org      string `json:"org,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Host     string `json:"host,omitempty"`
	CloneURL string `json:"cloneUrl,omitempty"`

	// PathAlias is the location under $parentDir/reviewbot-code/$org-$repo-$num/
	// where this repository is cloned. If this is not
	// set, $parentDir/reviewbot-code/$org-$repo-$num/repo will be
	// used as the default.
	PathAlias string `json:"pathAlias,omitempty"`
}

type GlobalConfig struct {
	// GithubReportType is the format of the report, will be used if linterConfig.GithubReportFormat is empty.
	// e.g. "github_checks", "github_pr_review"
	GithubReportType GithubReportType `json:"githubReportType,omitempty"`
	GitlabReportType GitlabReportType `json:"gitlabReportType,omitempty"`

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

	// CopySSHKeyToContainer is the path of the ssh key file to copy to the container.
	// optional, if not empty, copy the ssh key to the container.
	// it only works when using the docker runner.
	// format can be:
	// 1. /path/to/ssh/key => will copy the key to the same path(/path/to/ssh/key) in the container
	// 2. /path/to/ssh/key:/another/path/to/ssh/key => will copy the key to the target path(/another/path/to/ssh/key) in the container
	// it can be overridden by linter.DockerAsRunner.CopySSHKeyToContainer.
	CopySSHKeyToContainer string `json:"copySSHKeyToContainer,omitempty"`
}

// DockerAsRunner provides the way to run the linter using the docker.
type DockerAsRunner struct {
	Image                string `json:"image"`
	CopyLinterFromOrigin bool   `json:"copyLinterFromOrigin,omitempty"`
	// CopySSHKeyToContainer is the path of the ssh key file to copy to the container.
	// This key may be needed when go mod tidy to download the private repository or other similar cases.
	// optional, if not empty, copy the ssh key to the container.
	// format can be:
	// 1. /path/to/ssh/key => will copy the key to the same path(/path/to/ssh/key) in the container
	// 2. /path/to/ssh/key:/another/path/to/ssh/key => will copy the key to the target path(/another/path/to/ssh/key) in the container
	//
	// Note: it can not create the directory if the directory does not exist in the container.
	// see: https://github.com/moby/moby/issues/20920
	// so we need to ensure the directory exists in the container before copying the ssh key.
	CopySSHKeyToContainer string `json:"copySSHKeyToContainer,omitempty"`
}

// KubernetesAsRunner provides the way to run the linter using the kubernetes.
type KubernetesAsRunner struct {
	// Namespace is the namespace of the kubernetes.
	// if not set, use the default namespace.
	Namespace string `json:"namespace,omitempty"`
	// Image is the image to start the pod, which is used to run the linter.
	Image string `json:"image"`
	// CopySSHKeyToPod is the path of the ssh key file to copy to the pod.
	// This key may be needed when go mod tidy to download the private repository or other similar cases.
	// optional, if not empty, copy the ssh key to the pod.
	// format can be:
	// 1. /path/to/ssh/key => will copy the key to the same path(/path/to/ssh/key) in the pod
	// 2. /path/to/ssh/key:/another/path/to/ssh/key => will copy the key to the target path(/another/path/to/ssh/key) in the pod
	// The destination directory will be created via mounting a emptyDir volume in the pod.
	CopySSHKeyToPod string `json:"copySSHKeyToPod,omitempty"`
}

type IssueReference struct {
	// Pattern is the regex pattern to match the issue message.
	Pattern string `json:"pattern"`
	// URL is the url of the issue reference.
	URL string `json:"url"`
}

type CompiledIssueReference struct {
	Pattern *regexp.Regexp
	URL     string
}

type Linter struct {
	// Name is the linter name.
	Name string
	// Org is the organization which the linter will run on.
	Org string `json:"-"`
	// Repo is the repository which the linter will run on.
	Repo string `json:"-"`
	// Number is the number of PR or MR.
	Number int `json:"-"`
	// Workspace is the workspace to store all git repositories related by the linter.
	Workspace string `json:"-"`
	// Enable is whether to enable this linter, if false, linter still run but not report.
	Enable *bool `json:"enable,omitempty"`
	// DockerAsRunner is the docker image to run the linter.
	// Optional, if not empty, use the docker image to run the linter.
	// e.g. "golang:1.23.4"
	DockerAsRunner DockerAsRunner `json:"dockerAsRunner,omitempty"`
	// KubernetesAsRunner is the kubernetes pod to run the linter.
	// Optional, if not empty, use the kubernetes pod to run the linter.
	KubernetesAsRunner KubernetesAsRunner `json:"kubernetesAsRunner,omitempty"`
	// WorkDir is the working directory of the linter.
	WorkDir string `json:"workDir,omitempty"`
	// Command is the command to run the linter. e.g. "golangci-lint", "staticcheck"
	// If empty, use the linter name as the command.
	Command []string `json:"command,omitempty"`
	// Args is the arguments of the command.
	Args []string `json:"args,omitempty"`
	// Env is the environment variables required for the linter execution.
	Env []string `json:"env,omitempty"`

	// GithubReportFormat is the format of the report, if empty, use globalDefaultConfig.
	// For more details, see:
	// github_check_run: https://developer.github.com/v3/checks/runs/#create-a-check-run
	// github_pr_review: https://developer.github.com/v3/pulls/reviews/#create-a-pull-request-review
	// Note:
	// * github_check_run only support on Github Apps, not support on Github OAuth Apps or authenticated users.
	GithubReportFormat GithubReportType `json:"githubReportType,omitempty"`
	GitlabReportFormat GitlabReportType `json:"gitlabReportType,omitempty"`

	// ConfigPath is the path of the linter config file.
	// If not empty, use the config to run the linter.
	ConfigPath string `json:"configPath,omitempty"`

	// Modifier knowns how to modify the linter command.
	Modifier Modifier
}

func (l Linter) String() string {
	return fmt.Sprintf(
		"Linter{Enable: %v, DockerAsRunner: %v, Workspace: %v, WorkDir: %v, Command: %v, Args: %v, ReportFormat: %v, ConfigPath: %v}",
		*l.Enable, l.DockerAsRunner, l.Workspace, l.WorkDir, l.Command, l.Args, l.GithubReportFormat, l.ConfigPath)
}

var (
	ErrEmptyRepoOrOrg = errors.New("empty repo or org")
)

// NewConfig returns a new Config.
func NewConfig(conf string) (Config, error) {
	var c Config
	f, err := os.ReadFile(conf)
	if err != nil {
		return c, err
	}

	if err = yaml.UnmarshalStrict(f, &c); err != nil {
		return c, err
	}

	// ============ validate and update the config ============

	if err := c.parseCloneURLs(); err != nil {
		return c, err
	}
	if err := c.validateRefs(); err != nil {
		return c, err
	}
	if err := c.parseIssueReferences(); err != nil {
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

	// TODO(CarlJi): do we need to check the format of the copy ssh key here?
	log.Debugf("config %+v", c)

	return c, nil
}

func (c Config) GetLinterConfig(org, repo, ln string) Linter {
	linter := Linter{
		Enable:             boolPtr(true),
		GithubReportFormat: c.GlobalDefaultConfig.GithubReportType,
		GitlabReportFormat: c.GlobalDefaultConfig.GitlabReportType,
		Modifier:           NewBaseModifier(),
		Name:               ln,
		Org:                org,
		Repo:               repo,
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

	// set copy ssh key to container
	if c.GlobalDefaultConfig.CopySSHKeyToContainer != "" {
		linter.DockerAsRunner.CopySSHKeyToContainer = c.GlobalDefaultConfig.CopySSHKeyToContainer
	}

	if orgConfig, ok := c.CustomConfig[org]; ok {
		if l, ok := orgConfig.Linters[ln]; ok {
			linter = applyCustomConfig(linter, l)
		}
	}

	if repoConfig, ok := c.CustomConfig[org+"/"+repo]; ok {
		if l, ok := repoConfig.Linters[ln]; ok {
			linter = applyCustomConfig(linter, l)
		}
	}

	if linter.Command == nil {
		linter.Command = []string{ln}
	}

	return linter
}

// GetCompiledIssueReferences returns the compiled issue references config for the given linter name.
func (c Config) GetCompiledIssueReferences(linterName string) []CompiledIssueReference {
	if c.compiledIssueReferences == nil {
		return nil
	}
	if refs, ok := c.compiledIssueReferences[linterName]; ok {
		return refs
	}
	return nil
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

	if custom.GithubReportFormat != "" {
		legacy.GithubReportFormat = custom.GithubReportFormat
	}
	if custom.GitlabReportFormat != "" {
		legacy.GitlabReportFormat = custom.GitlabReportFormat
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
	if custom.DockerAsRunner.CopySSHKeyToContainer != "" {
		legacy.DockerAsRunner.CopySSHKeyToContainer = custom.DockerAsRunner.CopySSHKeyToContainer
	}

	if custom.KubernetesAsRunner.Image != "" {
		legacy.KubernetesAsRunner.Image = custom.KubernetesAsRunner.Image
	}
	if custom.KubernetesAsRunner.Namespace != "" {
		legacy.KubernetesAsRunner.Namespace = custom.KubernetesAsRunner.Namespace
	}
	if custom.KubernetesAsRunner.CopySSHKeyToPod != "" {
		legacy.KubernetesAsRunner.CopySSHKeyToPod = custom.KubernetesAsRunner.CopySSHKeyToPod
	}

	// if no namespace is set, use the default namespace
	if legacy.KubernetesAsRunner.Image != "" && legacy.KubernetesAsRunner.Namespace == "" {
		legacy.KubernetesAsRunner.Namespace = "default"
	}

	if custom.Name != "" {
		legacy.Name = custom.Name
	}

	return legacy
}

// GithubReportType is the type of the report.
type GithubReportType string
type GitlabReportType string

const (
	GithubCheckRuns            GithubReportType = "github_check_run"
	GithubPRReview             GithubReportType = "github_pr_review"
	GitlabComment              GitlabReportType = "gitlab_mr_comment"
	GitlabCommentAndDiscussion GitlabReportType = "gitlab_mr_comment_discussion"

	// for debug and testing.
	Quiet       GithubReportType = "quiet"
	QuietGitlab GitlabReportType = "quiet"
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

func (c *Config) parseCloneURLs() error {
	re := regexp.MustCompile(`^(?:git@|https://)?([^:/]+)[:/]{1}(.*?)/(.*?)\.git$`)

	for orgRepo, refConfig := range c.CustomConfig {
		for k, ref := range refConfig.Refs {
			if ref.CloneURL == "" {
				continue
			}

			if err := c.parseAndUpdateCloneURL(re, orgRepo, k); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Config) validateRefs() error {
	for orgRepo, refConfig := range c.CustomConfig {
		for _, ref := range refConfig.Refs {
			if ref.PathAlias != "" && (ref.Repo == "" || ref.Org == "") {
				log.Errorf("invalid ref: %v for org/repo: %s", ref, orgRepo)
				return ErrEmptyRepoOrOrg
			}
		}
	}

	return nil
}

func (c *Config) parseIssueReferences() error {
	if c.IssueReferences == nil {
		return nil
	}

	c.compiledIssueReferences = make(map[string][]CompiledIssueReference)

	for linterName, issueReferences := range c.IssueReferences {
		for _, ref := range issueReferences {
			re, err := regexp.Compile(ref.Pattern)
			if err != nil {
				return err
			}

			c.compiledIssueReferences[linterName] = append(c.compiledIssueReferences[linterName], CompiledIssueReference{
				Pattern: re,
				URL:     ref.URL,
			})
		}
	}

	return nil
}

func (c *Config) parseAndUpdateCloneURL(re *regexp.Regexp, orgRepo string, k int) error {
	ref := &c.CustomConfig[orgRepo].Refs[k]
	matches := re.FindStringSubmatch(ref.CloneURL)
	if len(matches) != 4 {
		log.Errorf("failed to parse CloneURL, please check the format of %s", ref.CloneURL)
		return errors.New("invalid CloneURL")
	}

	ref.Host = matches[1]
	ref.Org = matches[2]
	ref.Repo = matches[3]

	return nil
}
