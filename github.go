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

package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cr-bot/internal/linters"
	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/log"
)

// ListFiles lists all files for the specified pull request.
func (s *Server) ListPullRequestsFiles(ctx context.Context, owner string, repo string, number int) ([]*github.CommitFile, *github.Response, error) {
	opts := github.ListOptions{
		PerPage: 100,
	}

	var pullRequestAffectedFiles []*github.CommitFile

	for {
		files, response, err := s.gc.PullRequests.ListFiles(ctx, owner, repo, number, &opts)
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

func (s *Server) PostPullReviewCommentsWithRetry(ctx context.Context, owner string, repo string, number int, comments []*github.PullRequestComment) error {
	var existedComments []*github.PullRequestComment
	err := retryWithBackoff(ctx, func() error {
		originalComments, resp, err := s.gc.PullRequests.ListComments(ctx, owner, repo, number, nil)
		if err != nil {
			return err
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("list comments failed: %v", resp)
		}

		existedComments = originalComments
		return nil
	})

	if err != nil {
		return err
	}

	for _, comment := range comments {
		var existed bool
		for _, existedComment := range existedComments {
			if comment.GetPath() == existedComment.GetPath() &&
				comment.GetLine() == existedComment.GetLine() &&
				comment.GetBody() == existedComment.GetBody() {
				existed = true
				break
			}
		}

		if existed {
			continue
		}

		err := retryWithBackoff(ctx, func() error {
			_, resp, err := s.gc.PullRequests.CreateComment(ctx, owner, repo, number, comment)
			if err != nil {
				return err
			}
			if resp.StatusCode != 201 {
				return fmt.Errorf("create comment failed: %v", resp)
			}
			return nil
		})

		if err != nil {
			return err
		}

		log.Infof("create comment success: %v", comment)
	}
	return nil
}

func retryWithBackoff(ctx context.Context, f func() error) error {
	var backoff = time.Second
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

func GetCommitIDFromContentsURL(contentsURL string) (string, error) {
	// contentsURL: https://api.github.com/repos/owner/repo/contents/path/to/file?ref=commit_id
	// commitID: commit_id
	contentsURLRegex := regexp.MustCompile(`ref=(.*)$`)
	matches := contentsURLRegex.FindStringSubmatch(contentsURL)
	if len(matches) != 2 {
		return "", fmt.Errorf("invalid contentsURL: %s", contentsURL)
	}

	return matches[1], nil
}

func buildPullRequestCommentBody(linterName string, lintErrs map[string][]linters.LinterOutput, commitFiles []*github.CommitFile) ([]*github.PullRequestComment, error) {
	var comments []*github.PullRequestComment
	hunkChecker, err := NewGithubCommitFileHunkChecker(commitFiles)
	if err != nil {
		return nil, err
	}

	for _, commitFile := range commitFiles {
		file := commitFile.GetFilename()
		if commitFile.GetPatch() == "" {
			log.Debugf("empty patch, skipping %v\n", commitFile)
			continue
		}

		for lintFile, lintFileErrs := range lintErrs {
			if !strings.HasSuffix(file, lintFile) {
				log.Debugf("unrelated file, skipping %v", commitFile)
				continue
			}

			for _, lintErr := range lintFileErrs {
				if !hunkChecker.InHunk(file, lintErr.Line) {
					log.Debugf("lint err not in hunk, skipping %v, lintErr: %v", commitFile, lintErr)
					continue
				}

				commitID, err := GetCommitIDFromContentsURL(commitFile.GetContentsURL())
				if err != nil {
					log.Errorf("failed to get commit id from contents url, err: %v, url: %v", err, commitFile.GetContentsURL())
					return nil, err
				}

				message := fmt.Sprintf("[%s] %s",
					linterName, lintErr.Message)

				comments = append(comments, &github.PullRequestComment{
					Body:     github.String(message),
					Path:     github.String(file),
					Line:     github.Int(lintErr.Line),
					Side:     github.String("RIGHT"),
					CommitID: github.String(commitID),
				})
			}
		}
	}

	return comments, nil
}
