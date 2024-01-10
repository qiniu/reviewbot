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
	"context"
	"fmt"
	"regexp"

	"github.com/cr-bot/config"
	"github.com/cr-bot/internal/linters"
	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/log"
)

var lintName = "rebase-suggestion"

func init() {
	// register linter
	linters.RegisterCommentHandler(lintName, rebaseSuggestionHandler)
}

func rebaseSuggestionHandler(linterConfig config.Linter, agent linters.Agent, event github.PullRequestEvent) error {
	opts := &github.ListOptions{}
	commits, response, err := agent.GitHubClient().PullRequests.ListCommits(context.Background(), event.GetRepo().GetOwner().GetLogin(), event.GetRepo().GetName(), event.GetNumber(), opts)
	if err != nil {
		return err
	}

	if response.StatusCode != 200 {
		log.Errorf("list commits failed: %v", response)
		return fmt.Errorf("list commits failed: %v", response.Body)
	}

	comment := checkCommitMessage(commits)
	if len(comment) == 0 {
		return nil
	}
	c, resp, err := agent.GitHubClient().Issues.CreateComment(context.Background(), event.GetRepo().GetOwner().GetLogin(), event.GetRepo().GetName(), event.GetNumber(), &github.IssueComment{
		Body: &comment,
	})
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		log.Errorf("create comment failed: %v", resp)
		return fmt.Errorf("create comment failed: %v", resp.Body)
	}

	log.Infof("create comment success: %v", c)

	return nil
}

func checkCommitMessage(commits []*github.RepositoryCommit) string {
	pattern := `^Merge (.*) into (.*)$`
	reg := regexp.MustCompile(pattern)

	for _, commit := range commits {
		if commit.Commit != nil && commit.Commit.Message != nil {
			if reg.MatchString(*commit.Commit.Message) {
				return "please rebase your PR"
			}
		}
	}

	return ""
}
