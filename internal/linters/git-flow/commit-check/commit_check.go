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

package commit_check

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

const lintName = "commit-check"

func init() {
	linters.RegisterPullRequestHandler(lintName, commitMessageCheckHandler)
}

func commitMessageCheckHandler(log *xlog.Logger, a linters.Agent) error {
	var (
		org    = a.PullRequestEvent.GetRepo().GetOwner().GetLogin()
		repo   = a.PullRequestEvent.GetRepo().GetName()
		number = a.PullRequestEvent.GetNumber()
		author = a.PullRequestEvent.GetPullRequest().GetUser().GetLogin()
	)

	commits, err := listCommits(context.Background(), a, org, repo, number)
	if err != nil {
		return err
	}

	existedComments, err := listExistedComments(context.Background(), a, org, repo, number)
	if err != nil {
		return err
	}

	var toComments []string
	for _, rule := range rulers {
		var cmt string
		cmt, err := rule(log, commits)
		if err != nil {
			return err
		}

		if cmt != "" {
			toComments = append(toComments, cmt)
		}
	}

	return handle(context.Background(), log, a, org, repo, author, number, toComments, existedComments)
}

func listCommits(ctx context.Context, agent linters.Agent, org, repo string, number int) ([]*github.RepositoryCommit, error) {
	var preFilterCommits []*github.RepositoryCommit
	opts := &github.ListOptions{}
	commits, response, err := agent.GithubClient.PullRequests.ListCommits(context.Background(), org, repo, number, opts)
	if err != nil {
		return preFilterCommits, err
	}

	if response.StatusCode != 200 {
		log.Errorf("list commits failed: %v", response)
		return preFilterCommits, fmt.Errorf("list commits failed: %v", response.Body)
	}

	return commits, nil
}

func listExistedComments(ctx context.Context, agent linters.Agent, org, repo string, number int) ([]*github.IssueComment, error) {
	comments, resp, err := agent.GithubClient.Issues.ListComments(ctx, org, repo, number, &github.IssueListCommentsOptions{})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		log.Errorf("list comments failed: %v", resp)
		return nil, fmt.Errorf("list comments failed: %v", resp.Body)
	}

	return comments, nil
}

// Deprecated: this is old version of commit check, which is not used anymore. Remove this after a while.
const rebaseSuggestionFlag = "REBASE SUGGESTION"
const commitCheckFlag = "[Git-flow]"

const commentTmpl = `**{{.Flag}}** Hi @{{.Author}}, There are some suggestions for your information:

---

{{range	.Comments}}
> {{.}}
{{end}}
For other ` + "`" + `git-flow` + "`" + ` instructions, recommend refer to [these examples](https://github.com/qiniu/reviewbot/blob/master/docs/engineering-practice/git-flow-instructions_zh.md).

{{.Footer}}
`

type RebaseSuggestion struct {
	Author        string
	Flag          string
	TargetCommits []string
}

func handle(ctx context.Context, log *xlog.Logger, agent linters.Agent, org, repo, author string, number int, comments []string, existedComments []*github.IssueComment) error {
	var data = struct {
		Flag     string
		Author   string
		Comments []string
		Footer   string
	}{
		Flag:     commitCheckFlag,
		Author:   author,
		Comments: comments,
		Footer:   linters.CommentFooter,
	}

	tmpl, err := template.New("").Parse(commentTmpl)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		return err
	}

	expectedComment := buf.String()

	// check if comment already existed
	for _, comment := range existedComments {
		if comment.Body != nil && *comment.Body == expectedComment {
			// comment already existed, no need to create again
			return nil
		}
	}

	// remove comments with old rebaseSuggestion flag
	// TODO: remove this after a while
	for _, comment := range existedComments {
		if comment.Body != nil && strings.Contains(*comment.Body, rebaseSuggestionFlag) {
			_, err := agent.GithubClient.Issues.DeleteComment(context.Background(), org, repo, *comment.ID)
			if err != nil {
				log.Warnf("delete comment failed: %v", err)
				continue
			}
		}
	}

	// remove old comments
	for _, comment := range existedComments {
		if comment.Body != nil && strings.Contains(*comment.Body, commitCheckFlag) {
			_, err := agent.GithubClient.Issues.DeleteComment(context.Background(), org, repo, *comment.ID)
			if err != nil {
				log.Warnf("delete comment failed: %v", err)
				continue
			}
		}
	}

	// add new comment
	if len(comments) > 0 {
		c, resp, err := agent.GithubClient.Issues.CreateComment(context.Background(), org, repo, number, &github.IssueComment{
			Body: &expectedComment,
		})
		if err != nil {
			return err
		}
		if resp.StatusCode != 201 {
			log.Errorf("create comment failed: %v", resp)
			return fmt.Errorf("create comment failed: %v", resp.Body)
		}

		log.Infof("create comment success: %v", c)
	}

	return nil
}

// Ruler is a function to check if commit messages match some rules
// The message returned via Ruler will be added as part of the comment
// so, It's recommended to use template rulerTmpl to generate a unified format message
type Ruler func(log *xlog.Logger, commits []*github.RepositoryCommit) (string, error)

const rulerTmpl = `
### {{.Header}}
{{.Message}}
`

var rulers = map[string]Ruler{
	"Rebase suggestions": rebaseCheck,
}

const (
	pattern = `^Merge (.*) into (.*)$`
)

var mergeMsgRegex = regexp.MustCompile(pattern)

// RebaseCheckRule checks if there are merge commit messages or duplicate messages in the PR
// If there are, it will return a suggestion message to do git rebase
func rebaseCheck(log *xlog.Logger, commits []*github.RepositoryCommit) (string, error) {
	var mergeTypeCommits []string
	// filter out duplicated commit messages
	var msgs = make(map[string]int, 0)

	// filter out merge commit messages
	for _, commit := range commits {
		if commit.Commit != nil && commit.Commit.Message != nil {
			if mergeMsgRegex.MatchString(*commit.Commit.Message) {
				mergeTypeCommits = append(mergeTypeCommits, *commit.Commit.Message)
			}
			msgs[*commit.Commit.Message]++
		}
	}

	var finalComments string

	// check if there are merge commit messages
	if len(mergeTypeCommits) > 0 {
		var cmtsForMergeTypeCommits string
		for _, msg := range mergeTypeCommits {
			cmtsForMergeTypeCommits += fmt.Sprintf("\n  > %s\n", msg)
		}
		finalComments += fmt.Sprintf("* Following commits seems generated via `git merge` \n %s", cmtsForMergeTypeCommits)
	}

	// check if there are duplicated commit messages
	var duplicatedCommits string
	for msg, count := range msgs {
		if count > 1 {
			for i := 0; i < count; i++ {
				duplicatedCommits += fmt.Sprintf("\n  > %s \n", msg)
			}
		}
	}

	if duplicatedCommits != "" {
		finalComments += fmt.Sprintf("* Following commits have **duplicated** messages \n %s", duplicatedCommits)
	}

	if finalComments == "" {
		return "", nil
	}

	// add footer
	finalComments += "\n Which seems insignificant, recommend to use **`git rebase`** command to reorganize your PR. \n"

	tmpl, err := template.New("ruler").Parse(rulerTmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	var data = struct {
		Header  string
		Message string
	}{
		Header:  "Rebase suggestions",
		Message: finalComments,
	}
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
