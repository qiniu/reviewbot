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
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/log"
	gitlab "github.com/xanzy/go-gitlab"
)

// ListFiles lists all files for the specified pull request.
func ListMergeRequestsFiles(ctx context.Context, gc *gitlab.Client, owner string, repo string, pid int, number int) ([]*gitlab.MergeRequestDiff, *gitlab.Response, error) {

	files, response, err := gc.MergeRequests.GetMergeRequestChanges(pid, number, nil)
	if err != nil {
		return nil, nil, err
	}
	return files.Changes, response, err
}

func ListMergeRequestsComments(ctx context.Context, gc *gitlab.Client, owner string, repo string, number int, pid int) ([]*gitlab.Note, error) {
	var allComments []*gitlab.Note
	opts := gitlab.ListMergeRequestNotesOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	comments1, resp1, err1 := gc.Notes.ListMergeRequestNotes(pid, number, nil)
	fmt.Println(comments1)
	fmt.Println(resp1)
	fmt.Println(err1)
	err := RetryWithBackoff(ctx, func() error {
		comments, resp, err := gc.Notes.ListMergeRequestNotes(pid, number, &opts)
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
func MergeRequestSHA(ctx context.Context, gc *gitlab.Client, pid int, num int) (HeadSha string, BaseSha string, StartSha string, err error) {
	var mr *gitlab.MergeRequest
	mr, _, err = gc.MergeRequests.GetMergeRequest(pid, num, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("get mr head sha failed: %v", err)

	}

	return mr.DiffRefs.HeadSha, mr.DiffRefs.BaseSha, mr.DiffRefs.StartSha, nil
}

func CreateMergeReviewComments(ctx context.Context, gc *gitlab.Client, owner string, repo string, number int, comments []*gitlab.Note, pid int) ([]*gitlab.Note, error) {
	var addedComments []*gitlab.Note
	//gc.MergeRequests.GetMergeRequestDiffVersions()

	for _, comment := range comments {
		cmt := comment
		var opt gitlab.CreateMergeRequestNoteOptions
		var optd gitlab.CreateMergeRequestDiscussionOptions
		opt.Body = &cmt.Body
		optd.Body = &cmt.Body
		err := RetryWithBackoff(ctx, func() error {
			log.Infof("CREATE NOTE %v,%v", pid, number)
			cm, resp, err := gc.Notes.CreateMergeRequestNote(pid, number, &opt)

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
func CreateMergeReviewDiscussion(ctx context.Context, gc *gitlab.Client, owner string, repo string, number int, comments []*gitlab.CreateMergeRequestDiscussionOptions, pid int) ([]*gitlab.Discussion, error) {
	var addedComments []*gitlab.Discussion
	//gc.MergeRequests.GetMergeRequestDiffVersions()

	for _, comment := range comments {
		cmt := comment
		log.Infof("CreateMergeReviewDiscussion%v", comment)

		var optd gitlab.CreateMergeRequestDiscussionOptions

		optd.Body = cmt.Body
		err := RetryWithBackoff(ctx, func() error {
			log.Infof("CREATE NOTE %v,%v", pid, number)

			cm, resp, err := gc.Discussions.CreateMergeRequestDiscussion(pid, number, comment)
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
func DeleteMergeReviewComments(ctx context.Context, gc *gitlab.Client, owner, repo string, comments []*gitlab.Note, pid int, number int) error {
	for _, comment := range comments {
		cmt := comment
		err := RetryWithBackoff(ctx, func() error {
			resp, err := gc.Notes.DeleteMergeRequestNote(pid, number, cmt.ID)
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
func DeleteMergeReviewCommentsForGitLab(ctx context.Context, gc *gitlab.Client, owner, repo string, comments []*gitlab.Note, pid int, number int) error {
	for _, comment := range comments {
		cmt := comment
		err := RetryWithBackoff(ctx, func() error {
			resp, err := gc.Notes.DeleteMergeRequestNote(pid, number, cmt.ID)
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
func NewGitLabCommitFileHunkChecker(commitFiles []*gitlab.MergeRequestDiff) (*GithubCommitFileHunkChecker, error) {
	hunks := make(map[string][]Hunk)
	for _, commitFile := range commitFiles {
		if commitFile == nil || commitFile.NewPath == "" {
			continue
		}

		if commitFile.DeletedFile == true {
			continue
		}

		fileHunks, err := DiffHunksMerge(commitFile)
		if err != nil {
			return nil, err
		}

		v, ok := hunks[commitFile.NewPath]
		if ok {
			log.Warnf("duplicate commitFiles: %v, %v", commitFile, v)
			continue
		}

		hunks[commitFile.NewPath] = fileHunks
	}

	return &GithubCommitFileHunkChecker{
		Hunks: hunks,
	}, nil
}

func FilterMergeRequestWithCommit(ctx context.Context, gc *gitlab.Client, pid, owner, repo, headSha string) ([]*gitlab.MergeRequest, error) {
	var allPRs []*gitlab.MergeRequest
	var state = "opened"

	opt := &gitlab.ListProjectMergeRequestsOptions{
		State:          &state,
		AuthorUsername: &owner,

		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}
	for {
		prs, resp, err := gc.MergeRequests.ListProjectMergeRequests(pid, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull requests: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to list pull requests: %v", gitlab.Stringify(resp.Body))
		}

		for _, pr := range prs {
			if pr.SHA == headSha {
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

// filterPullRequestComments filters out the comments that are already posted by the bot.
func filterLinterOutputsForGitLab(outputs map[string][]LinterOutput, comments []*gitlab.Note) (toAdds map[string][]LinterOutput, toDeletes []*gitlab.Note) {
	toAdds = make(map[string][]LinterOutput)

	validComments := make(map[int64]struct{})
	for file, lintFileErrs := range outputs {
		for _, lintErr := range lintFileErrs {
			var found bool
			for _, comment := range comments {
				if comment.FileName == file && comment.Position.NewLine == lintErr.Line && strings.Contains(comment.Body, lintErr.Message) {
					found = true
					validComments[int64(comment.ID)] = struct{}{}
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
		if _, ok := validComments[int64(comment.ID)]; !ok {
			toDeletes = append(toDeletes, comment)
		}
	}
	return toAdds, toDeletes
}

func constructMergeRequestComments(linterOutputs map[string][]LinterOutput, linterName, commitID string, headSha string, baseSha string, startSha string) []*gitlab.Note {
	var comments []*gitlab.Note
	for file, outputs := range linterOutputs {
		for _, output := range outputs {

			message := fmt.Sprintf("%s %s\n%s",
				linterName, output.Message, CommentFooter)

			if output.StartLine != 0 {
				comments = append(comments, &gitlab.Note{
					Body:     message,
					CommitID: commitID,
					Position: &gitlab.NotePosition{
						NewPath:  file,
						NewLine:  output.StartLine,
						BaseSHA:  baseSha,
						HeadSHA:  headSha,
						StartSHA: startSha,
					},
				})
			} else {
				comments = append(comments, &gitlab.Note{
					Body: message,

					Position: &gitlab.NotePosition{
						NewPath:  file,
						BaseSHA:  baseSha,
						HeadSHA:  headSha,
						StartSHA: startSha,
						NewLine:  output.StartLine,
					},
					CommitID: commitID,
				})
			}
		}
	}
	return comments
}

func constructMergeRequestDiscussion(linterOutputs map[string][]LinterOutput, linterName, commitID string, headSha string, baseSha string, startSha string) []*gitlab.CreateMergeRequestDiscussionOptions {
	var comments []*gitlab.CreateMergeRequestDiscussionOptions
	var ptype string

	ptype = "text"
	for file, outputs := range linterOutputs {
		for _, output := range outputs {

			message := fmt.Sprintf("%s %s\n%s",
				linterName, output.Message, CommentFooter)

			if output.StartLine != 0 {
				comments = append(comments, &gitlab.CreateMergeRequestDiscussionOptions{
					Body:     &message,
					CommitID: &commitID,

					Position: &gitlab.PositionOptions{
						NewPath:      &file,
						NewLine:      &output.Line, //&output.StartLine,
						BaseSHA:      &baseSha,
						HeadSHA:      &headSha,
						StartSHA:     &startSha,
						PositionType: &ptype,
						OldPath:      &file,
						OldLine:      &output.Line,
					},
				})
			} else {
				comments = append(comments, &gitlab.CreateMergeRequestDiscussionOptions{
					Body: &message,

					Position: &gitlab.PositionOptions{
						NewPath:      &file,
						BaseSHA:      &baseSha,
						HeadSHA:      &headSha,
						StartSHA:     &startSha,
						NewLine:      &output.Line, //&output.StartLine,
						PositionType: &ptype,
						OldPath:      &file,
						OldLine:      &output.Line,
					},
					CommitID: &commitID,
				})
			}
		}
	}
	return comments
}

// FilterPullRequests filter full request by commit.
func FilterPullRequestsWithCommitd(ctx context.Context, gc *github.Client, owner, repo, headSha string) ([]*github.PullRequest, error) {
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
func ListPullRequestsCommentsd(ctx context.Context, gc *github.Client, owner string, repo string, number int) ([]*github.PullRequestComment, error) {
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
func CreatePullReviewCommentsd(ctx context.Context, gc *github.Client, owner string, repo string, number int, comments []*github.PullRequestComment) ([]*github.PullRequestComment, error) {
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
func DeletePullReviewCommentsd(ctx context.Context, gc *github.Client, owner, repo string, comments []*github.PullRequestComment) error {
	log := lintersutil.FromContext(ctx)
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
func CreateGithubChecksd(ctx context.Context, a Agent, lintErrs map[string][]LinterOutput) (*github.CheckRun, error) {
	var (
		headSha    = a.PullRequestEvent.GetPullRequest().GetHead().GetSHA()
		owner      = a.PullRequestEvent.Repo.GetOwner().GetLogin()
		repo       = a.PullRequestEvent.Repo.GetName()
		startTime  = a.PullRequestEvent.GetPullRequest().GetUpdatedAt()
		linterName = a.LinterConfig.Name
	)
	log := lintersutil.FromContext(ctx)
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
		log.Debugf("Log view :%s", logURL)
		check.Output.Summary = github.String(fmt.Sprintf("This is [the detailed log](%s).\n\n%s", logURL, Reference))
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

func GetCommitIDFromContentsUR(contentsURL string) (string, error) {
	// contentsURL: https://api.github.com/repos/owner/repo/contents/path/to/file?ref=commit_id
	// commitID: commit_id
	matches := contentsURLRegex.FindStringSubmatch(contentsURL)
	if len(matches) != 2 {
		return "", fmt.Errorf("invalid contentsURL: %s", contentsURL)
	}

	return matches[1], nil
}

func linterNamePrefi(linterName string) string {
	return fmt.Sprintf("[%s]", linterName)
}

func constructPullRequestComment(linterOutputs map[string][]LinterOutput, linterName, commitID string) []*github.PullRequestComment {
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
func filterLinterOutput(outputs map[string][]LinterOutput, comments []*github.PullRequestComment) (toAdds map[string][]LinterOutput, toDeletes []*github.PullRequestComment) {
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

func toGithubCheckRunAnnotation(linterOutputs map[string][]LinterOutput) []*github.CheckRunAnnotation {
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
