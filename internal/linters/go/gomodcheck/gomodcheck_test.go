package gomodcheck

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

func TestGoModCheck(t *testing.T) {
	modDir := "c/go.mod"
	tcs := []struct {
		id      string
		content []byte
		input   linters.Agent
		want    map[string][]linters.LinterOutput
	}{
		{
			id:      "case1 : cross-repository local replacement ",
			content: []byte("replace github.com/xxxxx/xxx v0.0.0 => ../../github.com/xxx/xxxx"),
			input: linters.Agent{
				RepoDir: "a/b",
				PullRequestChangedFiles: []*github.CommitFile{
					{
						Filename: &modDir,
					},
				},
			},
			want: map[string][]linters.LinterOutput{
				"c/go.mod": {
					{
						File:    modDir,
						Line:    1,
						Column:  1,
						Message: "cross-repository local replacement are not allowed[reviewbot]\nfor more information see https://github.com/qiniu/reviewbot/issues/275",
					},
				},
			},
		},
		{
			id:      "case2 : valid local replacement ",
			content: []byte("replace github.com/xxxxx/xxx v0.0.0 => ../github.com/xxx/xxxx"),
			input: linters.Agent{
				RepoDir: "a/b",
				PullRequestChangedFiles: []*github.CommitFile{
					{
						Filename: &modDir,
					},
				},
			},
			want: map[string][]linters.LinterOutput{},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.id, func(t *testing.T) {
			filename := filepath.Join(tc.input.RepoDir, modDir)
			err := os.MkdirAll(filepath.Dir(filename), 0755)
			if err != nil {
				t.Errorf("Error creating directories:%v", err)
				return
			}
			err = os.WriteFile(filename, tc.content, 0644)
			if err != nil {
				t.Errorf("Error writing to file: %v", err)
				return
			}

			defer func() {
				err = os.RemoveAll(filename)
				if err != nil {
					t.Errorf("Error writing to file: %v", err)
					return
				}
			}()

			output, err := goModCheckOutput(&xlog.Logger{}, tc.input)
			if err != nil {
				t.Errorf("Error execute goModCheckOutput : %v", err)
			}
			if !reflect.DeepEqual(output, tc.want) {
				t.Errorf("got output = %v, want = %v", output, tc.want)
			}

		})
	}
}

func TestIsSubstring(t *testing.T) {
	tcs := []struct {
		id   string
		A    string
		subB string
		want bool
	}{
		{
			id:   "case1 : subB is not a subdir of A",
			A:    "A/B/C",
			subB: "A/B/C/../",
			want: false,
		},
		{
			id:   "case2 : subB is not a subdir of A",
			A:    "A/B/C",
			subB: "A/B/C/D/../",
			want: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.id, func(t *testing.T) {
			got, _ := isSubdirectory(tc.A, tc.subB)
			if got != tc.want {
				t.Errorf("isSubdirectory() = %v, want = %v", got, tc.want)
			}
		})
	}
}
