package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	testCases := []struct {
		name        string
		expectError bool
		rawConfig   string
		expected    Config
	}{
		{
			name:        "no config",
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
		{
			name:        "config with golangci-lint config path",
			expectError: false,
			rawConfig: `
globalDefaultConfig: # global default settings, will be overridden by qbox org and repo specific settings if they exist
  githubReportType: "github_check_run" # github_pr_review, github_check_run
  golangciLintConfig: "linters-config/.golangci.yml"

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
					GithubReportType:   GithubCheckRuns,
					GolangCiLintConfig: "linters-config/.golangci.yml",
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
			if err := os.WriteFile(configPath, []byte(tc.rawConfig), 0o600); err != nil {
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

				if !strings.HasSuffix(c.GlobalDefaultConfig.GolangCiLintConfig, tc.expected.GlobalDefaultConfig.GolangCiLintConfig) {
					t.Errorf("expected %v, got %v", tc.expected.GlobalDefaultConfig.GolangCiLintConfig, c.GlobalDefaultConfig.GolangCiLintConfig)
				}

				if !reflect.DeepEqual(c.CustomConfig, tc.expected.CustomConfig) {
					t.Errorf("expected %v, got %v", tc.expected.CustomConfig, c.CustomConfig)
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
  golangciLintConfig: "linters-config/.golangci.yml"

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
  
  qbox/net-common:	  
    golangci-lint:
      enable: true
      args: []
      configPath: "repo.golangci.yml"

  qbox/net-tools:	  
    golangci-lint:
      enable: false
  
  qbox/kodo-ops:
    golangci-lint:
      enable: true
      command:
        - "/bin/sh"
        - "-c"
        - "--"
      args:
        - |
          cd website && yarn build && cd ..
          golangci-lint run --enable-all --timeout=5m0s --allow-parallel-runners=true --print-issued-lines=false --out-format=line-number >> $ARTIFACT/lint.log 2>&1

`

	if err := os.WriteFile(configPath, []byte(rawConfig), 0o600); err != nil {
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
				Command: []string{"luacheck"},
			},
		},
		{
			name:   "case2",
			org:    "qbox",
			repo:   "net-cache",
			linter: "golangci-lint",
			want: Linter{
				Enable:     boolPtr(true),
				Args:       []string{"run", "-D", "staticcheck"},
				ConfigPath: "linters-config/.golangci.yml",
				Command:    []string{"golangci-lint"},
			},
		},
		{
			name:   "case3",
			org:    "qbox",
			repo:   "net-cache",
			linter: "staticcheck",
			want: Linter{
				Enable:  boolPtr(true),
				Command: []string{"staticcheck"},
			},
		},
		{
			name:   "case4",
			org:    "qbox",
			repo:   "net-gslb",
			linter: "staticcheck",
			want: Linter{
				Enable:  boolPtr(true),
				Command: []string{"staticcheck"},
			},
		},
		{
			name:   "case5",
			org:    "qiniu",
			repo:   "net-gslb",
			linter: "staticcheck",
			want: Linter{
				Enable:  boolPtr(true),
				Command: []string{"staticcheck"},
			},
		},
		{
			name:   "case6",
			org:    "qbox",
			repo:   "net-gslb",
			linter: "golangci-lint",
			want: Linter{
				Enable:     boolPtr(true),
				Args:       []string{"run", "-D", "staticcheck"},
				ConfigPath: "linters-config/.golangci.yml",
				Command:    []string{"golangci-lint"},
			},
		},
		{
			name:   "case7",
			org:    "qiniu",
			repo:   "net-gslb",
			linter: "golangci-lint",
			want: Linter{
				Enable:     boolPtr(true),
				ConfigPath: "linters-config/.golangci.yml",
				Command:    []string{"golangci-lint"},
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
				Command: []string{"staticcheck"},
			},
		},
		{
			name:   "case9 - repo configuration will override the default global configurations.",
			org:    "qbox",
			repo:   "net-common",
			linter: "golangci-lint",
			want: Linter{
				Enable:     boolPtr(true),
				ConfigPath: "repo.golangci.yml",
				Args:       []string{},
				Command:    []string{"golangci-lint"},
			},
		},
		{
			name:   "case10 - turn off golangci-lint for net-tools",
			org:    "qbox",
			repo:   "net-tools",
			linter: "golangci-lint",
			want: Linter{
				Enable:  boolPtr(false),
				Command: []string{"golangci-lint"},
				Args:    []string{"run", "-D", "staticcheck"},
			},
		},
		{
			name:   "case11 - complex script",
			org:    "qbox",
			repo:   "kodo-ops",
			linter: "golangci-lint",
			want: Linter{
				Enable:  boolPtr(true),
				Command: []string{"/bin/sh", "-c", "--"},
				Args: []string{
					"cd website && yarn build && cd ..\ngolangci-lint run --enable-all --timeout=5m0s --allow-parallel-runners=true --print-issued-lines=false --out-format=line-number >> $ARTIFACT/lint.log 2>&1\n",
				},
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

			if len(got.Command) != len(tc.want.Command) || !reflect.DeepEqual(got.Command, tc.want.Command) {
				t.Errorf("expected %v, got %v", tc.want.Command, got.Command)
			}

			if len(got.Args) != len(tc.want.Args) || !reflect.DeepEqual(got.Args, tc.want.Args) {
				t.Errorf("args expected %v, got %v", tc.want.Args, got.Args)
			}

			if !strings.HasSuffix(got.ConfigPath, tc.want.ConfigPath) {
				t.Errorf("expected %v, got %v", tc.want.ConfigPath, got.ConfigPath)
			}
		})
	}
}
