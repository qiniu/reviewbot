package lint

import (
	"context"
	"reflect"
	"regexp"
	"testing"

	"github.com/qiniu/reviewbot/config"
)

func TestApplyTypedMessageByIssueReferences(t *testing.T) {
	// Mock issue reference pattern
	pattern := regexp.MustCompile(`#(\d+)`)
	pattern2 := regexp.MustCompile(`#ABC`)

	testCases := []struct {
		name           string
		reportFormat   config.ReportType
		lintResults    map[string][]LinterOutput
		issueRefs      []config.CompiledIssueReference
		expectedOutput map[string][]LinterOutput
	}{
		{
			name:         "GithubCheckRuns format",
			reportFormat: config.GitHubCheckRuns,
			lintResults: map[string][]LinterOutput{
				"file1.go": {
					{
						Message: "variable naming issue #123",
						Line:    10,
					},
				},
			},
			issueRefs: []config.CompiledIssueReference{
				{
					Pattern:     pattern,
					URL:         "https://github.com/qiniu/reviewbot/issues/wrongissue",
					IssueNumber: 123,
				},
			},
			expectedOutput: map[string][]LinterOutput{
				"file1.go": {
					{
						Message:      "variable naming issue #123",
						TypedMessage: "variable naming issue #123\nmore info: https://github.com/qiniu/reviewbot/issues/wrongissue",
						Line:         10,
					},
				},
			},
		},
		{
			name:         "GithubPRReview format",
			reportFormat: config.GitHubPRReview,
			lintResults: map[string][]LinterOutput{
				"file2.go": {
					{
						Message: "function complexity issue #456",
						Line:    20,
					},
				},
			},
			issueRefs: []config.CompiledIssueReference{
				{
					Pattern:     pattern,
					URL:         "https://github.com/qiniu/reviewbot/issues/wrongissue",
					IssueNumber: 0,
				},
			},
			expectedOutput: map[string][]LinterOutput{
				"file2.go": {
					{
						Message:      "function complexity issue #456",
						TypedMessage: "[function complexity issue #456](https://github.com/qiniu/reviewbot/issues/wrongissue)",
						Line:         20,
					},
				},
			},
		},
		{
			name:         "GithubMixType format",
			reportFormat: config.GitHubMixType,
			lintResults: map[string][]LinterOutput{
				"file3.go": {
					{
						Message: "code style issue #789",
						Line:    30,
					},
				},
			},
			issueRefs: []config.CompiledIssueReference{
				{
					Pattern:     pattern,
					URL:         "https://github.com/qiniu/reviewbot/issues/wrongissue",
					IssueNumber: 0,
				},
			},
			expectedOutput: map[string][]LinterOutput{
				"file3.go": {
					{
						Message:      "code style issue #789",
						TypedMessage: "[code style issue #789](https://github.com/qiniu/reviewbot/issues/wrongissue)",
						Line:         30,
					},
				},
			},
		},
		{
			name:         "No matching issue reference",
			reportFormat: config.GitHubCheckRuns,
			lintResults: map[string][]LinterOutput{
				"file4.go": {
					{
						Message: "regular lint message without issue reference",
						Line:    40,
					},
				},
			},
			issueRefs: []config.CompiledIssueReference{
				{
					Pattern:     pattern,
					URL:         "https://github.com/qiniu/reviewbot/issues/999",
					IssueNumber: 0,
				},
			},
			expectedOutput: map[string][]LinterOutput{
				"file4.go": {
					{
						Message: "regular lint message without issue reference",
						Line:    40,
					},
				},
			},
		},
		{
			name:         "Multiple files and issues",
			reportFormat: config.GitHubMixType,
			lintResults: map[string][]LinterOutput{
				"file5.go": {
					{
						Message: "issue #111",
						Line:    50,
					},
					{
						Message: "issue #ABC",
						Line:    51,
					},
				},
				"file6.go": {
					{
						Message: "no issue reference",
						Line:    60,
					},
				},
			},
			issueRefs: []config.CompiledIssueReference{
				{
					Pattern:     pattern,
					URL:         "https://github.com/qiniu/reviewbot/issues/wrongissue",
					IssueNumber: 0,
				},
				{
					Pattern:     pattern2,
					URL:         "https://github.com/qiniu/reviewbot/issues/wrongissue",
					IssueNumber: 0,
				},
			},
			expectedOutput: map[string][]LinterOutput{
				"file5.go": {
					{
						Message:      "issue #111",
						TypedMessage: "[issue #111](https://github.com/qiniu/reviewbot/issues/wrongissue)",
						Line:         50,
					},
					{
						Message:      "issue #ABC",
						TypedMessage: "[issue #ABC](https://github.com/qiniu/reviewbot/issues/wrongissue)",
						Line:         51,
					},
				},
				"file6.go": {
					{
						Message: "no issue reference",
						Line:    60,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create agent with test configuration
			agent := &Agent{
				LinterConfig:    config.Linter{ReportType: tc.reportFormat},
				IssueReferences: tc.issueRefs,
			}

			// Create context
			ctx := context.Background()

			// Call the method
			result := agent.ApplyTypedMessageByIssueReferences(ctx, tc.lintResults)

			// Compare results
			if !reflect.DeepEqual(result, tc.expectedOutput) {
				t.Errorf("Expected %+v, got %+v", tc.expectedOutput, result)
			}
		})
	}
}
