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

func TestFindGoModsPaths(t *testing.T) {
	filename1 := "test1/test2/test3/a.go"
	filename2 := "a.go"
	filename3 := "test1/test2/a.go"
	filename4 := "test2/test2/a.go"
	tcs := []struct {
		id            string
		currentDir    string
		input         linters.Agent
		wantdir       string
		createmodPath []string
		isGoMod       bool
		gomodfilenum  int
	}{
		{
			id: "1 gomod file ",
			input: linters.Agent{
				LinterConfig: config.Linter{
					WorkDir: "repo1/",
				},
				PullRequestChangedFiles: []*github.CommitFile{
					{
						Filename: &filename1,
					},
					{
						Filename: &filename2,
					},
					{
						Filename: &filename3,
					},
				},
				RepoDir: "repo1/",
			},
			createmodPath: []string{
				"test1/go.mod",
			},
			gomodfilenum: 1,
			isGoMod:      true,
		},

		{
			id: "no gomod file",
			input: linters.Agent{
				LinterConfig: config.Linter{
					WorkDir: "repo2/",
				},
				PullRequestChangedFiles: []*github.CommitFile{
					{
						Filename: &filename1,
					},
					{
						Filename: &filename2,
					},
					{
						Filename: &filename3,
					},
				},
				RepoDir: "repo2/",
			},
			// createmodPath: "test1/go.mod",
			gomodfilenum: 0,
			isGoMod:      false,
		},
		{
			id: "3 gomod file in different dir, 1 gomod is not in filepath",
			input: linters.Agent{
				LinterConfig: config.Linter{
					WorkDir: "repo3/",
				},
				PullRequestChangedFiles: []*github.CommitFile{
					{
						Filename: &filename1,
					},
					{
						Filename: &filename2,
					},
					{
						Filename: &filename4,
					},
				},
				RepoDir: "repo3/",
			},
			createmodPath: []string{
				"test1/go.mod",
				"test2/go.mod",
				"go.mod",
				"test3/go.mod",
			},
			gomodfilenum: 3,
			isGoMod:      true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.id, func(t *testing.T) {
			if err := os.MkdirAll(tc.input.LinterConfig.WorkDir, 0777); err != nil {
				t.Errorf("failed to create workdir: %v", err)
			}
			defer os.RemoveAll(tc.input.LinterConfig.WorkDir)
			if tc.isGoMod {
				for _, gomodpath := range tc.createmodPath {
					path := tc.input.LinterConfig.WorkDir + gomodpath
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
			}

			gomodpaths := findGoMods(tc.input)

			if len(gomodpaths) != tc.gomodfilenum {
				t.Errorf("gomodpath num is err, got:%d ,want: %d,\n gomodpath: %v", len(gomodpaths), tc.gomodfilenum, gomodpaths)
			}
		})
	}

}
