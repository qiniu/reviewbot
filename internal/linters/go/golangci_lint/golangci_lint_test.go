package golangcilint

import (
	"errors"
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

func TestParser(t *testing.T) {
	cases := []struct {
		output []byte
		want   map[string][]linters.LinterOutput
		err    error
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
			err: nil,
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
			err: nil,
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
			err: nil,
		},
	}

	for _, tt := range cases {
		got, err := parser(xlog.New("UnitTest"), tt.output)
		if !errors.Is(err, tt.err) {
			t.Errorf("parser() error = %v, wantErr %v", err, tt.err)
			return
		}

		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parser() = %v, want %v", got, tt.want)
		}
	}
}
