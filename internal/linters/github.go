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

package linters

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/log"
)

// ListFiles lists all files for the specified pull request.
func ListPullRequestsFiles(ctx context.Context, gc *github.Client, owner string, repo string, number int) ([]*github.CommitFile, *github.Response, error) {
	opts := github.ListOptions{
		PerPage: 100,
	}

	var pullRequestAffectedFiles []*github.CommitFile

	for {
		files, response, err := gc.PullRequests.ListFiles(ctx, owner, repo, number, &opts)
		if err != nil {
			return nil, nil, err
		}

		pullRequestAffectedFiles = append(pullRequestAffectedFiles, files...)

		if response.NextPage == 0 {
			return pullRequestAffectedFiles, response, nil
		}

		opts.Page++
	}
}

// FilterPullRequests filter full request by commit.
func FilterPullRequestsWithCommit(ctx context.Context, gc *github.Client, owner, repo, headSha string) ([]*github.PullRequest, error) {
	var allPRs []*github.PullRequest
	opt := &github.PullRequestListOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		prs, resp, err := gc.PullRequests.List(ctx, owner, repo, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull requests: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to list pull requests: %v", github.Stringify(resp.Body))
		}

		for _, pr := range prs {
			if pr.GetHead().GetSHA() == headSha {
				allPRs = append(allPRs, pr)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allPRs, nil
}

// ListPullRequestsComments lists all comments on the specified pull request.
// TODO(CarlJi): add pagination support.
func ListPullRequestsComments(ctx context.Context, gc *github.Client, owner string, repo string, number int) ([]*github.PullRequestComment, error) {
	var allComments []*github.PullRequestComment
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	err := RetryWithBackoff(ctx, func() error {
		comments, resp, err := gc.PullRequests.ListComments(ctx, owner, repo, number, opts)
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("list comments failed: %v", resp)
		}

		allComments = comments
		return nil
	})

	return allComments, err
}

// CreatePullReviewComments creates the specified comments on the pull request.
func CreatePullReviewComments(ctx context.Context, gc *github.Client, owner string, repo string, number int, comments []*github.PullRequestComment) ([]*github.PullRequestComment, error) {
	var addedComments []*github.PullRequestComment
	for _, comment := range comments {
		cmt := comment
		err := RetryWithBackoff(ctx, func() error {
			cm, resp, err := gc.PullRequests.CreateComment(ctx, owner, repo, number, cmt)
			if err != nil {
				return err
			}
			if resp.StatusCode != 201 {
				return fmt.Errorf("create comment failed: %v", resp)
			}
			addedComments = append(addedComments, cm)
			return nil
		})
		if err != nil {
			return nil, err
		}

		log.Infof("create comment success: %v", comment)
	}

	return addedComments, nil
}

// DeletePullReviewComments deletes the specified comments on the pull request.
func DeletePullReviewComments(ctx context.Context, gc *github.Client, owner, repo string, comments []*github.PullRequestComment) error {
	for _, comment := range comments {
		cmt := comment
		err := RetryWithBackoff(ctx, func() error {
			resp, err := gc.PullRequests.DeleteComment(ctx, owner, repo, cmt.GetID())
			if err != nil {
				return err
			}
			if resp.StatusCode != 204 {
				return fmt.Errorf("delete comment failed: %v", resp)
			}
			return nil
		})
		if err != nil {
			return err
		}

		log.Infof("delete comment success: %v", comment)
	}

	return nil
}

// CreateGithubChecks creates github checks for the specified pull request.
func CreateGithubChecks(ctx context.Context, a Agent, lintErrs map[string][]LinterOutput) (*github.CheckRun, error) {
	var (
		headSha    = a.PullRequestEvent.GetPullRequest().GetHead().GetSHA()
		owner      = a.PullRequestEvent.Repo.GetOwner().GetLogin()
		repo       = a.PullRequestEvent.Repo.GetName()
		startTime  = a.PullRequestEvent.GetPullRequest().GetUpdatedAt()
		linterName = a.LinterConfig.Name
	)

	annotations := toGithubCheckRunAnnotations(lintErrs)
	// limit the number of annotations to 50
	// see: https://github.com/qiniu/reviewbot/issues/258
	if len(annotations) > 50 {
		annotations = annotations[:50]
	}
	check := github.CreateCheckRunOptions{
		Name:      linterName,
		HeadSHA:   headSha,
		Status:    github.String("completed"),
		StartedAt: &startTime,
		CompletedAt: &github.Timestamp{
			Time: time.Now(),
		},
		Output: &github.CheckRunOutput{
			Title:       github.String(fmt.Sprintf("%s found %d issues related to your changes", linterName, len(annotations))),
			Annotations: annotations,
		},
	}

	logURL := a.GenLogViewUrl()
	if logURL == "" {
		check.Output.Summary = github.String(Reference)
	} else {
		check.Output.Summary = github.String(fmt.Sprintf("[full log ](%s)\n", logURL) + Reference)
	}

	if len(annotations) > 0 {
		check.Conclusion = github.String("failure")
	} else {
		check.Conclusion = github.String("success")
	}

	var ch *github.CheckRun
	err := RetryWithBackoff(ctx, func() error {
		checkRun, resp, err := a.GithubClient.Checks.CreateCheckRun(ctx, owner, repo, check)
		if err != nil {
			log.Errorf("create check run failed: %v", err)
			return err
		}

		if resp.StatusCode != http.StatusCreated {
			log.Errorf("unexpected response when create check run: %v", resp)
			return fmt.Errorf("create check run failed: %v", resp)
		}

		ch = checkRun
		return nil
	})

	return ch, err
}

// RetryWithBackoff retries the function with backoff.
func RetryWithBackoff(ctx context.Context, f func() error) error {
	backoff := time.Second
	for i := 0; i < 5; i++ {
		err := f()
		if err == nil {
			return nil
		}

		log.Errorf("retrying after error: %v", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	return fmt.Errorf("failed after 5 retries")
}

var contentsURLRegex = regexp.MustCompile(`ref=(.*)$`)

func GetCommitIDFromContentsURL(contentsURL string) (string, error) {
	// contentsURL: https://api.github.com/repos/owner/repo/contents/path/to/file?ref=commit_id
	// commitID: commit_id
	matches := contentsURLRegex.FindStringSubmatch(contentsURL)
	if len(matches) != 2 {
		return "", fmt.Errorf("invalid contentsURL: %s", contentsURL)
	}

	return matches[1], nil
}

func linterNamePrefix(linterName string) string {
	return fmt.Sprintf("[%s]", linterName)
}

func constructPullRequestComments(linterOutputs map[string][]LinterOutput, linterName, commitID string) []*github.PullRequestComment {
	var comments []*github.PullRequestComment
	for file, outputs := range linterOutputs {
		for _, output := range outputs {

			message := fmt.Sprintf("%s %s\n%s",
				linterName, output.Message, CommentFooter)

			if output.StartLine != 0 {
				comments = append(comments, &github.PullRequestComment{
					Body:      github.String(message),
					Path:      github.String(file),
					Line:      github.Int(output.Line),
					StartLine: github.Int(output.StartLine),
					StartSide: github.String("RIGHT"),
					Side:      github.String("RIGHT"),
					CommitID:  github.String(commitID),
				})
			} else {
				comments = append(comments, &github.PullRequestComment{
					Body:     github.String(message),
					Path:     github.String(file),
					Line:     github.Int(output.Line),
					Side:     github.String("RIGHT"),
					CommitID: github.String(commitID),
				})
			}
		}
	}
	return comments
}

// filterPullRequestComments filters out the comments that are already posted by the bot.
func filterLinterOutputs(outputs map[string][]LinterOutput, comments []*github.PullRequestComment) (toAdds map[string][]LinterOutput, toDeletes []*github.PullRequestComment) {
	toAdds = make(map[string][]LinterOutput)

	validComments := make(map[int64]struct{})
	for file, lintFileErrs := range outputs {
		for _, lintErr := range lintFileErrs {
			var found bool
			for _, comment := range comments {
				if comment.GetPath() == file && comment.GetLine() == lintErr.Line && strings.Contains(comment.GetBody(), lintErr.Message) {
					found = true
					validComments[comment.GetID()] = struct{}{}
					break
				}
			}

			// if the linter err is not found, add it to the toAdds
			if !found {
				toAdds[file] = append(toAdds[file], lintErr)
			}
		}
	}

	// filter out the comments that are not in the linter outputs
	for _, comment := range comments {
		if _, ok := validComments[comment.GetID()]; !ok {
			toDeletes = append(toDeletes, comment)
		}
	}
	return toAdds, toDeletes
}

const Reference = "If you have any questions about this comment, feel free to [raise an issue here](https://github.com/qiniu/reviewbot)."

func toGithubCheckRunAnnotations(linterOutputs map[string][]LinterOutput) []*github.CheckRunAnnotation {
	var annotations []*github.CheckRunAnnotation
	for file, outputs := range linterOutputs {
		for _, output := range outputs {
			annotation := &github.CheckRunAnnotation{
				Path:            github.String(file),
				StartLine:       github.Int(output.Line),
				EndLine:         github.Int(output.Line),
				AnnotationLevel: github.String("warning"),
				Message:         github.String(output.Message),
			}
			annotations = append(annotations, annotation)
		}
	}
	return annotations
}
