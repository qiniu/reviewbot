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

package commit

import (
	"strings"
	"testing"

	"github.com/google/go-github/v57/github"
)

func TestRebaseCheckRule(t *testing.T) {
	tcs := []struct {
		title    string
		commits  []*github.RepositoryCommit
		expected string
	}{
		{
			title: "filter merge commits",
			commits: []*github.RepositoryCommit{
				{
					Commit: &github.Commit{
						Message: github.String("feat: add feature 1"),
					},
				},
				{
					Commit: &github.Commit{
						Message: github.String("Merge a into b"),
					},
				},
				{
					Commit: &github.Commit{
						Message: github.String("fix: fix bug 2"),
					},
				},
				{
					Commit: &github.Commit{
						Message: github.String("Merge xxx into xxx"),
					},
				},
			},
			expected: "git merge",
		},
		{
			title: "filter duplicate commits",
			commits: []*github.RepositoryCommit{
				{
					Commit: &github.Commit{
						Message: github.String("feat: add feature 1"),
					},
				},
				{
					Commit: &github.Commit{
						Message: github.String("feat: add feature 1"),
					},
				},
				{
					Commit: &github.Commit{
						Message: github.String("fix: fix bug 2"),
					},
				},
			},
			expected: "duplicated",
		},
		{
			title: "filter duplicate and merge commits",
			commits: []*github.RepositoryCommit{
				{
					Commit: &github.Commit{
						Message: github.String("feat: add feature 1"),
					},
				},
				{
					Commit: &github.Commit{
						Message: github.String("feat: add feature 1"),
					},
				},
				{
					Commit: &github.Commit{
						Message: github.String("Merge xxx into xxx"),
					},
				},
			},
			expected: "feat: add feature 1",
		},
		{
			title: "filter duplicate and merge commits",
			commits: []*github.RepositoryCommit{
				{
					Commit: &github.Commit{
						Message: github.String("feat: add feature 1"),
					},
				},
				{
					Commit: &github.Commit{
						Message: github.String("feat: add feature 2"),
					},
				},
			},
			expected: "",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			comments, err := rebaseCheck(nil, tc.commits)
			if err != nil {
				t.Fatal(err)
			}

			if tc.expected == "" && comments != "" {
				t.Fatalf("expected %s, got %s", tc.expected, comments)
			}

			if !strings.Contains(comments, tc.expected) {
				t.Fatalf("expected %s, got %s", tc.expected, comments)
			}
		})
	}
}
