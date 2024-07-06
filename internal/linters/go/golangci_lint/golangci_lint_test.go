package golangcilint

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

func TestParser(t *testing.T) {
	cases := []struct {
		id         string
		output     []byte
		want       map[string][]linters.LinterOutput
		unexpected []string
	}{
		{
			id: "case1 - no error",
			output: []byte(`
golangci_lint/golangci_lint.go:16:1: warning: (golint)
golangci_lint.go:18:3: error: (golint)
`),
			want: map[string][]linters.LinterOutput{
				"golangci_lint/golangci_lint.go": {
					{
						File:    "golangci_lint/golangci_lint.go",
						Line:    16,
						Column:  1,
						Message: "warning: (golint)",
					},
				},
				"golangci_lint.go": {
					{
						File:    "golangci_lint.go",
						Line:    18,
						Column:  3,
						Message: "error: (golint)",
					},
				},
			},
			unexpected: []string{},
		},
		{
			id: "case2 - with typecheck",
			output: []byte(`
golangci_lint.go:16:1: error (typecheck)
golangci_lint.go:16:1: error (gochecknoglobals)
`),
			want: map[string][]linters.LinterOutput{
				"golangci_lint.go": {
					{
						File:    "golangci_lint.go",
						Line:    16,
						Column:  1,
						Message: "error (gochecknoglobals)",
					},
				},
			},
			unexpected: []string{"golangci_lint.go:16:1: error (typecheck)"},
		},
		{
			id: "case3 - with warning",
			output: []byte(`
level=warning msg="[linters_context] copyloopvar: this linter is disabled because the Go version (1.18) of your project is lower than Go 1.22"
golangci_lint.go:16:1: warning: (gochecknoglobals)
`),

			want: map[string][]linters.LinterOutput{
				"golangci_lint.go": {
					{
						File:    "golangci_lint.go",
						Line:    16,
						Column:  1,
						Message: "warning: (gochecknoglobals)",
					},
				},
			},
			unexpected: []string{},
		},
	}

	for _, tt := range cases {
		t.Run(tt.id, func(t *testing.T) {
			got, unexpected := parser(xlog.New("ut"), tt.output)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parser() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(unexpected, tt.unexpected) {
				t.Errorf("parser() unexpected = %v, want %v", unexpected, tt.unexpected)
			}
		})
	}
}

func TestArgs(t *testing.T) {
	tp := true
	tcs := []struct {
		id    string
		input linters.Agent
		want  linters.Agent
	}{
		{
			id: "case1 - default args",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Enable:  &tp,
					Command: []string{"golangci-lint"},
				},
			},
			want: linters.Agent{
				LinterConfig: config.Linter{
					Enable:  &tp,
					Command: []string{"golangci-lint"},
					Args:    []string{"run", "--timeout=5m0s", "--allow-parallel-runners=true", "--out-format=line-number", "--print-issued-lines=false"},
				},
			},
		},
		{
			id: "case2 - custom args",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Enable:  &tp,
					Command: []string{"golangci-lint"},
					Args:    []string{"run", "--timeout=10m", "--out-format=tab", "--config", "golangci-lint.yml"},
				},
			},
			want: linters.Agent{
				LinterConfig: config.Linter{
					Enable:  &tp,
					Command: []string{"golangci-lint"},
					Args:    []string{"run", "--timeout=10m", "--out-format=tab", "--config", "golangci-lint.yml", "--allow-parallel-runners=true", "--print-issued-lines=false"},
				},
			},
		},
		{
			id: "case3 - custom command",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Command: []string{"bash"},
					Args:    []string{"run"},
				},
			},
			want: linters.Agent{
				LinterConfig: config.Linter{
					Command: []string{"bash"},
					Args:    []string{"run"},
				},
			},
		},
		{
			id: "case4 - not run command",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Command: []string{"golangci-lint"},
					Args:    []string{"linters"},
				},
			},
			want: linters.Agent{
				LinterConfig: config.Linter{
					Command: []string{"golangci-lint"},
					Args:    []string{"linters"},
				},
			},
		},
		{
			id: "case5 - shell command",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Command: []string{"/bin/bash", "-c", "--"},
					Args:    []string{"echo 'abc'", "golangci-lint run "},
				},
			},
			want: linters.Agent{
				LinterConfig: config.Linter{
					Command: []string{"/bin/bash", "-c", "--"},
					Args:    []string{"echo 'abc'", "golangci-lint run "},
				},
			},
		},
		{
			id: "case6 - custom config path",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Enable:     &tp,
					Command:    []string{"golangci-lint"},
					Args:       []string{"run"},
					ConfigPath: "config/golangci-lint.yml",
				},
			},
			want: linters.Agent{
				LinterConfig: config.Linter{
					Enable:     &tp,
					Command:    []string{"golangci-lint"},
					Args:       []string{"run", "--timeout=5m0s", "--allow-parallel-runners=true", "--out-format=line-number", "--print-issued-lines=false", "--config", "config/golangci-lint.yml"},
					ConfigPath: "config/golangci-lint.yml",
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.id, func(t *testing.T) {
			got := argsApply(xlog.New("ut"), tc.input)
			if !reflect.DeepEqual(got.LinterConfig, tc.want.LinterConfig) {
				t.Errorf("args() = %v, want %v", got.LinterConfig, tc.want.LinterConfig)
			}
		})
	}
}

func TestConfigApply(t *testing.T) {
	tcs := []struct {
		id         string
		input      linters.Agent
		wantSuffix string
	}{
		{
			id: "case1 - default config path",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Command:    []string{"golangci-lint"},
					Args:       []string{"run"},
					ConfigPath: ".golangci.yml",
				},
			},
			wantSuffix: ".golangci.yml",
		},
		{
			id: "case2 - with workdir",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Command:    []string{"golangci-lint"},
					Args:       []string{"run"},
					ConfigPath: ".golangci.yaml",
					WorkDir:    "testdata",
				},
			},
			wantSuffix: "testdata/.golangci.yml",
		},
		{
			id: "case3 - config outside repo",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Command:    []string{"golangci-lint"},
					Args:       []string{"run"},
					ConfigPath: "../../../../config/linters-config/.golangci.yml",
					WorkDir:    "/tmp",
				},
			},
			wantSuffix: "/tmp/.golangci.yml",
		},
		{
			id: "case4 - empty config path",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Command: []string{"golangci-lint"},
					Args:    []string{"run"},
				},
			},
			wantSuffix: "",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.id, func(t *testing.T) {
			if tc.input.LinterConfig.WorkDir != "" {
				if err := os.MkdirAll(tc.input.LinterConfig.WorkDir, 0o666); err != nil {
					t.Errorf("failed to create workdir: %v", err)
				}
			}

			got := configApply(xlog.New("ut"), tc.input)
			if !strings.HasSuffix(got, tc.wantSuffix) {
				t.Errorf("configApply() = %v, want with suffix %v", got, tc.wantSuffix)
			}
		})
	}
}

func TestWorkDirApply(t *testing.T) {
	filename := "test1/test2/test3/a.go"
	filename2 := "a.go"
	tcs := []struct {
		id            string
		currentDir    string
		input         linters.Agent
		wantdir       string
		createmodPath string
		isGoMod       bool
	}{
		{
			id: "workdir == repodir ,find go.mod in repo/",
			input: linters.Agent{
				LinterConfig: config.Linter{

					WorkDir: "repo1/",
				},
				PullRequestChangedFiles: []*github.CommitFile{
					{
						Filename: &filename,
					},
				},
				RepoDir: "repo1/",
			},
			createmodPath: "go.mod",
			wantdir:       "repo1",
			isGoMod:       true,
		},
		{
			id: "workdir == repodir, find go.mod in subdir (repo/test1/go.mod)",
			input: linters.Agent{
				LinterConfig: config.Linter{

					WorkDir: "repo2/",
				},
				PullRequestChangedFiles: []*github.CommitFile{
					{
						Filename: &filename,
					},
				},
				RepoDir: "repo2/",
			},
			createmodPath: "test1/go.mod",
			wantdir:       "repo2/test1",
			isGoMod:       true,
		},

		{
			id: "workdir == repodir, can't find go.mod ",
			input: linters.Agent{
				LinterConfig: config.Linter{
					WorkDir: "repo3/",
				},
				PullRequestChangedFiles: []*github.CommitFile{
					{
						Filename: &filename2,
					},
				},
				RepoDir: "repo3/",
			},
			createmodPath: "test1/go.mod",
			wantdir:       "repo3/",
			isGoMod:       false,
		},

		{
			id: "workdir == repodir,  find go.mod , len(filedirpaht)==1",
			input: linters.Agent{
				LinterConfig: config.Linter{
					WorkDir: "repo4/",
				},
				PullRequestChangedFiles: []*github.CommitFile{
					{
						Filename: &filename2,
					},
				},
				RepoDir: "repo4/",
			},
			createmodPath: "go.mod",
			wantdir:       "repo4",
			isGoMod:       true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.id, func(t *testing.T) {
			if err := os.MkdirAll(tc.input.LinterConfig.WorkDir, 0777); err != nil {
				t.Errorf("failed to create workdir: %v", err)
			}
			defer os.RemoveAll(tc.input.LinterConfig.WorkDir)
			if tc.isGoMod {
				path := tc.input.LinterConfig.WorkDir + tc.createmodPath
				err := os.MkdirAll(filepath.Dir(path), 0777)
				if err != nil {
					fmt.Println("Error creating directories:", err)
					return
				}
				file, err := os.Create(path)
				if err != nil {
					fmt.Println("Error creating file:", err)
					return
				}
				defer file.Close()

			}

			a := workDirApply(xlog.New("ur"), tc.input)
			if !strings.HasSuffix(a.LinterConfig.WorkDir, tc.wantdir) {
				t.Errorf("workDirApply() = %v, want with suffix %v", a.LinterConfig.WorkDir, tc.wantdir)
			}
			if !tc.isGoMod {
				if !strings.HasSuffix(a.LinterConfig.Env[0], "GO111MODULE=off") {
					t.Errorf("env = %v, want with suffix GO111MODULE=off", a.LinterConfig.Env[0])
				}
			}
		})
	}

}
