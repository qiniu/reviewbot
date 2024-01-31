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
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
	"github.com/reviewbot/config"
	"github.com/reviewbot/internal/linters"
)

const lintName = "commit-check"

func init() {
	linters.RegisterCommentHandler(lintName, commitMessageCheckHandler)
}

func commitMessageCheckHandler(log *xlog.Logger, linterConfig config.Linter, agent linters.Agent, event github.PullRequestEvent) error {
	var (
		org    = event.GetRepo().GetOwner().GetLogin()
		repo   = event.GetRepo().GetName()
		number = event.GetNumber()
		author = event.GetPullRequest().GetUser().GetLogin()
	)

	commits, err := listCommits(context.Background(), agent, org, repo, number)
	if err != nil {
		return err
	}

	existedComments, err := listExistedComments(context.Background(), agent, org, repo, number)
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

	return handle(context.Background(), log, agent, org, repo, author, number, toComments, existedComments)
}

const rebaseSuggestionFlag = "REBASE SUGGESTION"
const commitCheckFlag = "[Git-flow]"

const commentTmpl = `**{{.Flag}}** Hi @{{.Author}}, There are some suggestions for your information:

---

{{range	.Comments}}
> {{.}}
{{end}}

<details>

If you have any questions about this comment, feel free to raise an issue here:

- **https://github.com/qiniu/reviewbot/issues**

</details>
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
	}{
		Flag:     commitCheckFlag,
		Author:   author,
		Comments: comments,
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
			log.Infof("comment already existed: %v", comment.Body)
			return nil
		}
	}

	// remove comments with old rebaseSuggestion flag
	// TODO: remove this after a while
	for _, comment := range existedComments {
		if comment.Body != nil && strings.Contains(*comment.Body, rebaseSuggestionFlag) {
			_, err := agent.GitHubClient().Issues.DeleteComment(context.Background(), org, repo, *comment.ID)
			if err != nil {
				log.Warnf("delete comment failed: %v", err)
				continue
			}
		}
	}

	// remove old comments
	for _, comment := range existedComments {
		if comment.Body != nil && strings.Contains(*comment.Body, commitCheckFlag) {
			_, err := agent.GitHubClient().Issues.DeleteComment(context.Background(), org, repo, *comment.ID)
			if err != nil {
				log.Warnf("delete comment failed: %v", err)
				continue
			}
		}
	}

	// add new comment
	if len(comments) > 0 {
		c, resp, err := agent.GitHubClient().Issues.CreateComment(context.Background(), org, repo, number, &github.IssueComment{
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

func listCommits(ctx context.Context, agent linters.Agent, org, repo string, number int) ([]*github.RepositoryCommit, error) {
	var preFilterCommits []*github.RepositoryCommit
	opts := &github.ListOptions{}
	commits, response, err := agent.GitHubClient().PullRequests.ListCommits(context.Background(), org, repo, number, opts)
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
	comments, resp, err := agent.GitHubClient().Issues.ListComments(ctx, org, repo, number, &github.IssueListCommentsOptions{})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		log.Errorf("list comments failed: %v", resp)
		return nil, fmt.Errorf("list comments failed: %v", resp.Body)
	}

	return comments, nil
}

// Ruler is a function to check if commit messages match some rules
type Ruler func(log *xlog.Logger, commits []*github.RepositoryCommit) (string, error)

var rulers = map[string]Ruler{
	"Rebase suggestions": rebaseCheck,
	"Amend suggestions":  amendCheck,
}

const (
	pattern              = `^Merge (.*) into (.*)$`
	rebaseSuggestionTmpl = `
Your PR has **Merge** type commit messages like:
{{range	.TargetCommits}}
> {{.}}
{{end}}
Which seems insignificant, recommend to use` + ` **` + "`" + `git rebase <upstream>/<branch>` + "`" + `** command ` + `to reorganize your PR.
`
)

var mergeMsgRegex = regexp.MustCompile(pattern)

func rebaseCheck(log *xlog.Logger, commits []*github.RepositoryCommit) (string, error) {
	var preFilterCommits []string
	for _, commit := range commits {
		if commit.Commit != nil && commit.Commit.Message != nil {
			if mergeMsgRegex.MatchString(*commit.Commit.Message) {
				preFilterCommits = append(preFilterCommits, *commit.Commit.Message)
			}
		}
	}

	// no matched commit messages
	if len(preFilterCommits) == 0 {
		return "", nil
	}

	var data = struct {
		TargetCommits []string
	}{
		TargetCommits: preFilterCommits,
	}

	tmpl, err := template.New("").Parse(rebaseSuggestionTmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	suggestion := `
### Rebase suggestions
	` + buf.String()

	return suggestion, nil
}

const (
	amendSuggestionTmpl = `
Your PR has **duplicate** commit messages like:
{{range	.TargetCommits}}
> {{.}}
{{end}}
Recommend to use` + ` **` + "`" + `git rebase -i ` + "`" + `** command ` + `to reorganize your PR.
`
)

func amendCheck(log *xlog.Logger, commits []*github.RepositoryCommit) (string, error) {
	var msgs = make(map[string]int, 0)
	for _, commit := range commits {
		if commit.Commit != nil && commit.Commit.Message != nil {
			msgs[*commit.Commit.Message]++
		}
	}

	var preFilterCommits []string
	for msg, count := range msgs {
		if count > 1 {
			preFilterCommits = append(preFilterCommits, msg)
		}
	}

	if len(preFilterCommits) == 0 {
		// no duplicated commit messages
		return "", nil
	}

	var data = struct {
		TargetCommits []string
	}{
		TargetCommits: preFilterCommits,
	}

	tmpl, err := template.New("").Parse(amendSuggestionTmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	suggestion := `
### Amend suggestions
	` + buf.String()

	return suggestion, nil
}
