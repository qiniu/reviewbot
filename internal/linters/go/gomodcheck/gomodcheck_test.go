/*
 Copyright 2024 Qiniu Cloud (qiniu.com).

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package gomodcheck

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/internal/lint"
	"github.com/qiniu/x/xlog"
)

func TestGoModCheck(t *testing.T) {
	tcs := []struct {
		id      string
		content []byte
		input   []*github.CommitFile
		want    map[string][]lint.LinterOutput
	}{
		{
			id:      "case1 : cross-repository local replacement ",
			content: []byte("replace github.com/a/c v0.0.0 => ../../github.com/c/d"),
			input: []*github.CommitFile{
				{
					Filename: github.String("c/go.mod"),
				},
			},
			want: map[string][]lint.LinterOutput{
				"c/go.mod": {
					{
						File:    "c/go.mod",
						Line:    1,
						Column:  1,
						Message: "cross-repository local replacement are not allowed[reviewbot]\nfor more information see https://github.com/qiniu/reviewbot/issues/275",
					},
				},
			},
		},
		{
			id:      "case2 : valid local replacement ",
			content: []byte("replace github.com/a/b v0.0.0 => ../github.com/c/d"),
			input: []*github.CommitFile{
				{
					Filename: github.String("c/go.mod"),
				},
			},
			want: map[string][]lint.LinterOutput{},
		},
		{
			id:      "case3 : valid non-local replacement ",
			content: []byte("replace github.com/a/b v0.0.0 => github.com/c/d v1.1.1"),
			input: []*github.CommitFile{
				{
					Filename: github.String("c/go.mod"),
				},
			},
			want: map[string][]lint.LinterOutput{},
		},
		{
			id:      "case4 : multiple go.mod files",
			content: []byte("replace github.com/a/b v0.0.0 => ../../github.com/c/d"),
			input: []*github.CommitFile{
				{
					Filename: github.String("c/go.mod"),
				},
				{
					Filename: github.String("d/go.mod"),
				},
			},
			want: map[string][]lint.LinterOutput{
				"c/go.mod": {
					{
						File:    "c/go.mod",
						Line:    1,
						Column:  1,
						Message: "cross-repository local replacement are not allowed[reviewbot]\nfor more information see https://github.com/qiniu/reviewbot/issues/275",
					},
				},
				"d/go.mod": {
					{
						File:    "d/go.mod",
						Line:    1,
						Column:  1,
						Message: "cross-repository local replacement are not allowed[reviewbot]\nfor more information see https://github.com/qiniu/reviewbot/issues/275",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.id, func(t *testing.T) {
			p, err := lint.NewGithubProvider(context.TODO(), nil, github.PullRequestEvent{}, lint.WithPullRequestChangedFiles(tc.input))
			if err != nil {
				t.Errorf("Error creating github provider: %v", err)
				return
			}
			// prepare go.mod files
			for _, file := range p.GetFiles(nil) {
				filename := file
				dir := filepath.Dir(filename)
				err := os.MkdirAll(dir, 0o755)
				if err != nil {
					t.Errorf("Error creating directories: %v", err)
					return
				}
				defer os.RemoveAll(dir)

				err = os.WriteFile(filename, tc.content, 0o600)
				if err != nil {
					t.Errorf("Error writing to file: %v", err)
					return
				}
			}

			output, err := goModCheckOutput(&xlog.Logger{}, lint.Agent{
				Provider: p,
			})
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
