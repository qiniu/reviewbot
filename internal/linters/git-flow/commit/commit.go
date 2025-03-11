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
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/qiniu/reviewbot/internal/lint"
	"github.com/qiniu/reviewbot/internal/util"
)

const lintName = "commit-check"

func init() {
	lint.RegisterPullRequestHandler(lintName, commitMessageCheckHandler)
	// support all languages
	lint.RegisterLinterLanguages(lintName, []string{"*"})
}

func commitMessageCheckHandler(ctx context.Context, a lint.Agent) error {
	log := util.FromContext(ctx)
	if a.CLIMode {
		fmt.Println("CLI-info:skip to run commit-check")
		return nil
	}
	info := a.Provider.GetCodeReviewInfo()
	var (
		org    = info.Org
		repo   = info.Repo
		number = info.Number
		author = info.Author
	)

	commits, err := a.Provider.ListCommits(ctx, org, repo, number)
	if err != nil {
		return err
	}

	existedComments, err := a.Provider.ListComments(ctx, org, repo, number)
	if err != nil {
		return err
	}
	log.Infof("existedComments: %d", len(existedComments))
	var toComments []string
	for _, rule := range rulers {
		var cmt string
		cmt, err := rule(ctx, commits)
		if err != nil {
			return err
		}

		if cmt != "" {
			toComments = append(toComments, cmt)
		}
	}
	return handle(ctx, a, org, repo, author, number, toComments, existedComments)
}

const (
	commitCheckFlag = "[Git-flow]"
)

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

func handle(ctx context.Context, agent lint.Agent, org, repo, author string, number int, comments []string, existedComments []lint.Comment) error {
	log := util.FromContext(ctx)
	data := struct {
		Flag     string
		Author   string
		Comments []string
		Footer   string
	}{
		Flag:     commitCheckFlag,
		Author:   author,
		Comments: comments,
		Footer:   lint.CommentFooter,
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

	var commentsExist bool
	for _, comment := range existedComments {
		if !commentsExist && comment.Body == expectedComment {
			commentsExist = true
			continue
		}

		// remove old comments
		if strings.Contains(comment.Body, commitCheckFlag) {
			err := agent.Provider.DeleteComment(ctx, org, repo, comment.ID)
			if err != nil {
				log.Warnf("delete comment failed: %v", err)
				continue
			}
		}
	}

	if commentsExist {
		return nil
	}

	// add new comment
	if len(comments) > 0 {
		c, err := agent.Provider.CreateComment(ctx, org, repo, number, &lint.Comment{
			Body: expectedComment,
		})
		if err != nil {
			return err
		}
		log.Infof("create comment success: %v", c.HTMLURL)
	}

	return nil
}

// Ruler is a function to check if commit messages match some rules.
// The message returned via Ruler will be added as part of the comment.
// So, It's recommended to use template rulerTmpl to generate a unified format message.
type Ruler func(ctx context.Context, commits []lint.Commit) (string, error)

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

// RebaseCheckRule checks if there are merge commit messages or duplicate messages in the PR.
// If there are, it will return a suggestion message to do git rebase.
func rebaseCheck(ctx context.Context, commits []lint.Commit) (string, error) {
	var mergeTypeCommits []string
	// filter out duplicated commit messages
	msgs := make(map[string]int, 0)

	// filter out merge commit messages
	for _, commit := range commits {
		if mergeMsgRegex.MatchString(commit.Message) {
			mergeTypeCommits = append(mergeTypeCommits, commit.Message)
		}
		msgs[commit.Message]++
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
	data := struct {
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
