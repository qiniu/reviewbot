package golangcilint

import (
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

func TestParser(t *testing.T) {
	cases := []struct {
		output     []byte
		want       map[string][]linters.LinterOutput
		unexpected []string
	}{
		{
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
			unexpected: nil,
		},
		{
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
			unexpected: nil,
		},
	}

	for _, tt := range cases {
		got, unexpected := parser(xlog.New("UnitTest"), tt.output)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parser() = %v, want %v", got, tt.want)
		}
		if !reflect.DeepEqual(unexpected, tt.unexpected) {
			t.Errorf("unexpected = %v, want %v", unexpected, tt.unexpected)
		}
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
			got := args(tc.input)
			if !reflect.DeepEqual(got.LinterConfig, tc.want.LinterConfig) {
				t.Errorf("args() = %v, want %v", got.LinterConfig, tc.want.LinterConfig)
			}
		})
	}
}
