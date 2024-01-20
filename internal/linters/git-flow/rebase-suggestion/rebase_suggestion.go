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

package rebase_suggestion

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/reviewbot/config"
	"github.com/reviewbot/internal/linters"
	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/log"
)

var lintName = "rebase-suggestion"

func init() {
	linters.RegisterCommentHandler(lintName, rebaseSuggestionHandler)
}

func rebaseSuggestionHandler(linterConfig config.Linter, agent linters.Agent, event github.PullRequestEvent) error {
	var (
		org    = event.GetRepo().GetOwner().GetLogin()
		repo   = event.GetRepo().GetName()
		number = event.GetNumber()
	)

	preFilterCommits, err := listMatchedCommitMessages(context.Background(), agent, org, repo, number)
	if err != nil {
		return err
	}

	existedComments, err := listExistedComments(context.Background(), agent, org, repo, number)
	if err != nil {
		return err
	}

	return handle(context.Background(), agent, org, repo, number, preFilterCommits, existedComments)
}

var rebaseSuggestionFlag = "**[REBASE SUGGESTION]**"

var rebaseSuggestionTmpl = rebaseSuggestionFlag + ` This PR has commit message like:
{{range	.}}
> {{.}}
{{end}}

Which seems insignificant, recommend to use ` + "`git rebase <upstream> <branch>`" + `to reorganize your PR. 

<details>

If you have any questions about this comment, feel free to raise an issue here:

- **https://github.com/qiniu/reviewbot/issues**

</details>
 `

func handle(ctx context.Context, agent linters.Agent, org, repo string, number int, prefilterCommits []*github.RepositoryCommit, existedComments []*github.IssueComment) error {
	var commitMessages []string
	for _, commit := range prefilterCommits {
		commitMessages = append(commitMessages, *commit.Commit.Message)
	}

	tmpl, err := template.New("rebase-suggestion").Parse(rebaseSuggestionTmpl)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, commitMessages); err != nil {
		return err
	}

	var expectedComment = buf.String()

	// check if comment already existed
	for _, comment := range existedComments {
		if comment.Body != nil && *comment.Body == expectedComment {
			log.Infof("comment already existed: %v", comment.Body)
			return nil
		}
	}

	// remove old comments
	for _, comment := range existedComments {
		if comment.Body != nil && strings.Contains(*comment.Body, rebaseSuggestionFlag) {
			_, err := agent.GitHubClient().Issues.DeleteComment(context.Background(), org, repo, *comment.ID)
			if err != nil {
				log.Warnf("delete comment failed: %v", err)
				continue
			}
		}
	}

	// add new comment
	if len(commitMessages) > 0 {
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

func listMatchedCommitMessages(ctx context.Context, agent linters.Agent, org, repo string, number int) ([]*github.RepositoryCommit, error) {
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

	pattern := `^Merge (.*) into (.*)$`
	reg := regexp.MustCompile(pattern)

	for _, commit := range commits {
		if commit.Commit != nil && commit.Commit.Message != nil {
			if reg.MatchString(*commit.Commit.Message) {
				preFilterCommits = append(preFilterCommits, commit)
			}
		}
	}

	return preFilterCommits, nil
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
