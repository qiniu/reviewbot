package commit_check

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
