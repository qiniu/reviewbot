package linters

import (
	"context"
	"time"
)

// Provider is the interface implemented by each git provider. such as github, gitlab, etc.
// It is responsible for interacting with the git provider when reviewing the PR/MR.
type Provider interface {
	// IsRelated returns whether the issue is related to the file changed.
	// file/line/startLine represent the issue location.
	IsRelated(file string, line int, startLine int) bool
	// HandleComments handles the comments for the linter.
	// Base on the linter outputs, the provider will create or update or delete the comments for the PR/MR.
	HandleComments(ctx context.Context, outputs map[string][]LinterOutput) error
	// Report reports the lint results to the provider.
	Report(ctx context.Context, agent Agent, lintResults map[string][]LinterOutput) error
	// GetFiles returns the files that match the given predicate in the PR/MR.
	// if predicate is nil, it returns all the files except removed files in the PR/MR.
	// NOTE(CarlJi): this is a simplified definition since only the file path is returned.
	// In the future, we may need more file status information.
	GetFiles(predicate func(filepath string) bool) []string

	GetToken() Token

	// ListCommits lists the commits in the PR/MR.
	ListCommits(ctx context.Context, org, repo string, number int) ([]Commit, error)
	// ListComments lists the comments in the PR/MR.
	ListComments(ctx context.Context, org, repo string, number int) ([]Comment, error)
	// DeleteComment deletes the comment in the PR/MR.
	DeleteComment(ctx context.Context, org, repo string, commentID int64) error
	// CreateComment creates a comment in the PR/MR.
	CreateComment(ctx context.Context, org, repo string, number int, comment *Comment) (*Comment, error)

	// GetCodeReviewInfo gets the code review information for the PR/MR.
	GetCodeReviewInfo() CodeReview
}

// Commit represents a Git commit.
type Commit struct {
	Message string `json:"message,omitempty"`
}

// Comment represents a comment left on an issue or a PR/MR.
type Comment struct {
	ID        int64     `json:"id,omitempty"`
	Body      string    `json:"body,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	URL       string    `json:"url,omitempty"`
	HTMLURL   string    `json:"html_url,omitempty"`
	IssueURL  string    `json:"issue_url,omitempty"`
}

// CodeReview has the information of a PR/MR.
type CodeReview struct {
	Org       string    `json:"org,omitempty"`
	Repo      string    `json:"repo,omitempty"`
	Number    int       `json:"number,omitempty"`
	URL       string    `json:"url,omitempty"`
	Author    string    `json:"author,omitempty"`
	HeadSHA   string    `json:"head_sha,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}
