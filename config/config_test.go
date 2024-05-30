package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	var testCases = []struct {
		name        string
		expectError bool
		rawConfig   string
		expected    Config
	}{
		{
			name:        "default config",
			expectError: false,
			rawConfig:   ``,
			expected: Config{
				GlobalDefaultConfig: GlobalConfig{
					GithubReportType: GithubPRReview,
				},
			},
		},
		{
			name:        "valid config",
			expectError: false,
			rawConfig: `
globalDefaultConfig: # global default settings, will be overridden by qbox org and repo specific settings if they exist
  githubReportType: "github_check_run" # github_pr_review, github_check_run

customConfig: # custom config for specific orgs or repos
  qbox: # github organization name
    golangci-lint:
      enable: true
      args: ["run", "-D", "staticcheck"] # disable staticcheck globally since we have a separate linter for it

  qbox/net-cache:
    luacheck:
      enable: true
      workDir: "nginx" # only run in the nginx directory since there are .luacheckrc files in this directory
`,
			expected: Config{
				GlobalDefaultConfig: GlobalConfig{
					GithubReportType: GithubCheckRuns,
				},
				CustomConfig: map[string]map[string]Linter{
					"qbox": {
						"golangci-lint": {
							Enable: boolPtr(true),
							Args:   []string{"run", "-D", "staticcheck"},
						},
					},
					"qbox/net-cache": {
						"luacheck": {
							Enable:  boolPtr(true),
							WorkDir: "nginx",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configDir := t.TempDir()
			configPath := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tc.rawConfig), 0666); err != nil {
				t.Fatalf("fail to write prow config: %v", err)
			}
			defer os.Remove(configPath)

			c, err := NewConfig(configPath)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}

				if c.GlobalDefaultConfig.GithubReportType != tc.expected.GlobalDefaultConfig.GithubReportType {
					t.Errorf("expected %v, got %v", tc.expected.GlobalDefaultConfig.GithubReportType, c.GlobalDefaultConfig.GithubReportType)
				}

				if len(c.CustomConfig) != len(tc.expected.CustomConfig) {
					t.Errorf("expected %v, got %v", len(tc.expected.CustomConfig), len(c.CustomConfig))
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	rawConfig := `
globalDefaultConfig:
  githubReportType: "github_check_run" # github_pr_review, github_check_run

customConfig: # custom config for specific orgs or repos
  qbox: # github organization name
    golangci-lint:
      enable: true
      args: ["run", "-D", "staticcheck"] # disable staticcheck globally since we have a separate linter for it

  qbox/net-cache:
    luacheck:
      enable: true
      workDir: "nginx" # only run in the nginx directory since there are .luacheckrc files in this directory

  qbox/kodo:
    staticcheck:
      enable: true
      workDir: "src/qiniu.com/kodo"
`

	if err := os.WriteFile(configPath, []byte(rawConfig), 0666); err != nil {
		t.Fatalf("fail to write prow config: %v", err)
	}
	defer os.Remove(configPath)

	c, err := NewConfig(configPath)
	if err != nil {
		t.Fatalf("fail to create config: %v, err:%v", configPath, err)
	}

	tcs := []struct {
		name   string
		org    string
		repo   string
		linter string
		want   Linter
	}{
		{
			name:   "case1",
			org:    "qbox",
			repo:   "net-cache",
			linter: "luacheck",
			want: Linter{
				Enable:  boolPtr(true),
				WorkDir: "nginx",
			},
		},
		{
			name:   "case2",
			org:    "qbox",
			repo:   "net-cache",
			linter: "golangci-lint",
			want: Linter{
				Enable: boolPtr(true),
				Args:   []string{"run", "-D", "staticcheck"},
			},
		},
		{
			name:   "case3",
			org:    "qbox",
			repo:   "net-cache",
			linter: "staticcheck",
			want: Linter{
				Enable: boolPtr(true),
			},
		},
		{
			name:   "case4",
			org:    "qbox",
			repo:   "net-gslb",
			linter: "staticcheck",
			want: Linter{
				Enable: boolPtr(true),
			},
		},
		{
			name:   "case5",
			org:    "qiniu",
			repo:   "net-gslb",
			linter: "staticcheck",
			want: Linter{
				Enable: boolPtr(true),
			},
		},
		{
			name:   "case6",
			org:    "qbox",
			repo:   "net-gslb",
			linter: "golangci-lint",
			want: Linter{
				Enable: boolPtr(true),
				Args:   []string{"run", "-D", "staticcheck"},
			},
		},
		{
			name:   "case7",
			org:    "qiniu",
			repo:   "net-gslb",
			linter: "golangci-lint",
			want: Linter{
				Enable: boolPtr(true),
				Args:   []string{},
			},
		},
		{
			name:   "case8",
			org:    "qbox",
			repo:   "kodo",
			linter: "staticcheck",
			want: Linter{
				Enable:  boolPtr(true),
				WorkDir: "src/qiniu.com/kodo",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := c.Get(tc.org, tc.repo, tc.linter)
			if *got.Enable != *tc.want.Enable {
				t.Errorf("expected %v, got %v", *tc.want.Enable, *got.Enable)
			}

			if got.WorkDir != tc.want.WorkDir {
				t.Errorf("expected %v, got %v", tc.want.WorkDir, got.WorkDir)
			}

			if len(got.Args) != len(tc.want.Args) {
				t.Errorf("expected %v, got %v", tc.want.Args, got.Args)
			}
		})
	}
}
