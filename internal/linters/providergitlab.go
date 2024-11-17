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
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/reviewbot/internal/metric"
	"github.com/qiniu/x/errors"
	"github.com/qiniu/x/log"
	gitlab "github.com/xanzy/go-gitlab"
	gitv2 "sigs.k8s.io/prow/pkg/git/v2"
)

var (
	errCreateComment = errors.New("failed to create comment")
	errMrHeadSha     = errors.New("get mr head sha failed")
	errListComment   = errors.New("failed to list MR comment")
	errDeleteComment = errors.New("failed to delete MR comment")
)

type DiffSha struct {
	HeadSha  string
	BaseSha  string
	StartSha string
}

func ListMergeRequestsFiles(ctx context.Context, gc *gitlab.Client, owner string, repo string, pid int, number int) ([]*gitlab.MergeRequestDiff, *gitlab.Response, error) {
	// For version compatibility,because verion below 10.8 not support ListMergeRequestDiffs
	files, response, err := gc.MergeRequests.GetMergeRequestChanges(pid, number, nil)
	if err != nil {
		return nil, nil, err
	}
	return files.Changes, response, err
}

func (g *GitlabProvider) ListCommits(ctx context.Context, org, repo string, number int) ([]Commit, error) {
	log := lintersutil.FromContext(ctx)
	opts := &gitlab.GetMergeRequestCommitsOptions{
		PerPage: 100,
	}
	var allCommits []Commit

	for {
		commits, resp, err := g.GitLabClient.MergeRequests.GetMergeRequestCommits(g.MergeRequestEvent.ObjectAttributes.TargetProjectID, number, opts)
		if err != nil {
			return nil, fmt.Errorf("listing commits: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Errorf("list commits failed: %v", resp)
			return nil, ErrListCommits
		}

		for _, commit := range commits {
			allCommits = append(allCommits, Commit{
				Message: commit.Message,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allCommits, nil
}

// make sure the GitLabProvider implements the Provider interface.
var _ Provider = (*GitlabProvider)(nil)

type GitlabProvider struct {
	// GitHubClient is the GitHub client.
	GitLabClient *gitlab.Client
	// GitClient is the Git client factory.
	GitClient gitv2.ClientFactory

	// HunkChecker is the hunk checker for the file.
	HunkChecker *FileHunkChecker

	// PullRequestChangedFiles is the changed files of a pull request.
	MergeRequestChangedFiles []*gitlab.MergeRequestDiff
	// PullRequestEvent is the event of a pull request.
	MergeRequestEvent gitlab.MergeEvent
}

func (g *GitlabProvider) ListComments(ctx context.Context, org, repo string, number int) ([]Comment, error) {
	var pid = g.MergeRequestEvent.ObjectAttributes.TargetProjectID
	opts := gitlab.ListMergeRequestNotesOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	comments, resp, err := g.GitLabClient.Notes.ListMergeRequestNotes(pid, number, &opts)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Errorf("list comments failed: %v", resp)
		return nil, ErrListComments
	}
	var allComments = make([]Comment, 0, len(comments))
	for _, comment := range comments {
		allComments = append(allComments, Comment{
			ID:        int64(comment.ID),
			Body:      comment.Body,
			CreatedAt: *comment.CreatedAt,
			UpdatedAt: *comment.UpdatedAt,
			URL:       "",
			HTMLURL:   "",
			IssueURL:  "",
		})
	}
	return allComments, nil
}

func (g *GitlabProvider) DeleteComment(ctx context.Context, org, repo string, commentID int64) error {
	_, err := g.GitLabClient.Notes.DeleteMergeRequestNote(g.MergeRequestEvent.ObjectAttributes.TargetProjectID, g.MergeRequestEvent.ObjectAttributes.IID, int(commentID), nil)
	return err
}

func (g *GitlabProvider) CreateComment(ctx context.Context, org, repo string, number int, comment *Comment) (*Comment, error) {
	cmt := comment
	var opt gitlab.CreateMergeRequestNoteOptions
	opt.Body = &cmt.Body
	cm, resp, err := g.GitLabClient.Notes.CreateMergeRequestNote(g.MergeRequestEvent.ObjectAttributes.TargetProjectID, number, &opt)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		log.Errorf("create comment failed: %v", resp)
		return nil, errCreateComment
	}
	return &Comment{
		ID:        int64(cm.ID),
		Body:      comment.Body,
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
		URL:       "",
		HTMLURL:   "",
		IssueURL:  "",
	}, nil
}

func (g *GitlabProvider) GetCodeReviewInfo() CodeReview {
	updatetime, _ := time.Parse("2006-01-02 15:04:05", g.MergeRequestEvent.ObjectAttributes.UpdatedAt)
	return CodeReview{
		Org:       g.MergeRequestEvent.Project.Namespace,
		Repo:      g.MergeRequestEvent.Repository.Name,
		Number:    g.MergeRequestEvent.ObjectAttributes.IID,
		Author:    g.MergeRequestEvent.ObjectAttributes.LastCommit.Author.Name,
		URL:       g.MergeRequestEvent.ObjectAttributes.LastCommit.URL,
		HeadSHA:   g.MergeRequestEvent.ObjectAttributes.LastCommit.ID,
		UpdatedAt: updatetime,
	}
}

func NewGitlabProvider(gitlabClient *gitlab.Client, gitClient gitv2.ClientFactory, mergeRequestChangedFiles []*gitlab.MergeRequestDiff, mergeRequestEvent gitlab.MergeEvent) (*GitlabProvider, error) {
	checker, err := NewGitLabCommitFileHunkChecker(mergeRequestChangedFiles)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	if err != nil {
		return nil, err
	}
	return &GitlabProvider{
		GitLabClient:             gitlabClient,
		GitClient:                gitClient,
		MergeRequestChangedFiles: mergeRequestChangedFiles,
		MergeRequestEvent:        mergeRequestEvent,
		HunkChecker:              checker,
	}, nil
}

func ReportFormMatCheck(gc *gitlab.Client, reportFormat config.ReportType) (reportType config.ReportType) {
	//  gitlab  verion below 10.8 not support discussion resource api
	//  gitlab reportformart  not config:  version=>10.8: GitlabCommentAndDiscussion, version<10.8: GitlabComment
	//  gitlab reportformart  config:  version=>10.8: use config, version<10.8: GitlabComment
	v, r, e := gc.Version.GetVersion()
	if e != nil {
		log.Fatalf("Failed to get version: %v,response is %v", e, r)
		return config.Quiet
	}
	v1, _ := version.NewVersion(v.Version)
	v2, _ := version.NewVersion("10.8")
	if reportFormat != "" {
		if v1.LessThan(v2) {
			return config.GitlabComment
		}
		return reportFormat
	}
	if v1.LessThan(v2) {
		return config.GitlabComment
	}
	return config.GitlabCommentAndDiscussion
}

func (g *GitlabProvider) HandleComments(ctx context.Context, outputs map[string][]LinterOutput) error {
	return nil
}

func linterNamePrefixGitLab(linterName string) string {
	return fmt.Sprintf("[**%s**]", linterName)
}

func (g *GitlabProvider) Report(ctx context.Context, a Agent, lintResults map[string][]LinterOutput) error {
	linterName := a.LinterConfig.Name
	org := a.Provider.GetCodeReviewInfo().Org
	repo := a.Provider.GetCodeReviewInfo().Repo
	num := a.Provider.GetCodeReviewInfo().Number
	orgRepo := fmt.Sprintf("%s/%s", org, repo)
	reportformat := ReportFormMatCheck(g.GitLabClient, a.LinterConfig.ReportType)
	switch reportformat {
	case config.GitlabCommentAndDiscussion:
		// list   MR  comments
		var pid = g.MergeRequestEvent.ObjectAttributes.TargetProjectID
		existedComments, err := g.ListMergeRequestsComments(ctx, g.GitLabClient, org, repo, num, pid)
		if err != nil {
			log.Errorf("failed to list comments: %v", err)
			return err
		}
		// filter out the comments that are not related to the linter
		var existedCommentsToKeep []*gitlab.Note
		linterFlag := linterNamePrefixGitLab(linterName)
		for _, comment := range existedComments {
			if strings.HasPrefix(comment.Body, linterFlag) {
				existedCommentsToKeep = append(existedCommentsToKeep, comment)
			}
		}
		log.Infof("%s found %d existed comments for this PR %d (%s) \n", linterFlag, len(existedCommentsToKeep), num, orgRepo)
		toDeletes := existedCommentsToKeep
		// delete comments that are  related to the linter
		if err := DeleteMergeReviewCommentsForGitLab(ctx, g.GitLabClient, org, repo, toDeletes, pid, num); err != nil {
			log.Errorf("failed to delete comments: %v", err)
			return err
		}
		log.Infof("%s delete %d comments for this PR %d (%s) \n", linterFlag, len(toDeletes), num, orgRepo)
		h, e := MergeRequestSHA(g.GitLabClient, pid, num)

		if e != nil {
			log.Errorf("failed to delete comments: %v", e)
			return e
		}
		// create  MR  comments form linter result
		logURL := a.GenLogViewURL()
		commerr := CreateGitLabCommentsReport(ctx, g.GitLabClient, lintResults, linterName, pid, num, logURL)
		if commerr != nil {
			log.Errorf("failed to delete comments: %v", commerr)
		}
		// list discussions
		dlist, err := ListMergeRequestDiscussions(ctx, g.GitLabClient, num, pid)
		if err != nil {
			log.Errorf("failed to list comments: %v", err)
			return err
		}
		// delete discussion  that are related the linter
		errd := DeleteMergeRequestDiscussions(ctx, g.GitLabClient, num, pid, dlist, linterFlag)
		if errd != nil {
			log.Errorf("failed to delete discussion: %v", err)
			return errd
		}
		// construct  discussion  from lint result
		discussion := constructMergeRequestDiscussion(lintResults, linterFlag, g.MergeRequestEvent.ObjectAttributes.LastCommit.ID, h.HeadSha, h.BaseSha, h.StartSha)
		if len(discussion) == 0 {
			return nil
		}
		log.Infof("discussion%v", discussion)
		// add discussion
		addedDis, err := CreateMergeReviewDiscussion(ctx, g.GitLabClient, org, repo, num, discussion, pid)
		if err != nil {
			log.Errorf("failed to post discussions: %v", err)
			return err
		}
		log.Infof("[%s] add %d comments for this PR %d (%s) \n", linterName, len(addedDis), num, orgRepo)
		metric.NotifyWebhookByText(ConstructGotchaMsg(linterName, g.MergeRequestEvent.Project.WebURL, "", lintResults))
	case config.GitlabComment:
		var pid = g.MergeRequestEvent.ObjectAttributes.TargetProjectID
		existedComments, err := g.ListMergeRequestsComments(ctx, g.GitLabClient, org, repo, num, pid)
		if err != nil {
			log.Errorf("failed to list comments: %v", err)
			return err
		}
		// filter out the comments that are not related to the linter
		var existedCommentsToKeep []*gitlab.Note
		linterFlag := linterNamePrefixGitLab(linterName)
		for _, comment := range existedComments {
			if strings.HasPrefix(comment.Body, linterFlag) {
				existedCommentsToKeep = append(existedCommentsToKeep, comment)
			}
		}
		log.Infof("%s found %d existed comments for this PR %d (%s) \n", linterFlag, len(existedCommentsToKeep), num, orgRepo)

		if err := DeleteMergeReviewCommentsForGitLab(ctx, g.GitLabClient, org, repo, existedCommentsToKeep, pid, num); err != nil {
			log.Errorf("failed to delete comments: %v", err)
			return err
		}
		log.Infof("%s delete %d comments for this PR %d (%s) \n", linterFlag, len(existedCommentsToKeep), num, orgRepo)
		logURL := a.GenLogViewURL()
		commerr := CreateGitLabCommentsReport(ctx, g.GitLabClient, lintResults, linterName, pid, num, logURL)
		if commerr != nil {
			log.Errorf("failed to post comments: %v", commerr)
		}
		metric.NotifyWebhookByText(ConstructGotchaMsg(linterName, g.MergeRequestEvent.Project.WebURL, "", lintResults))
	case config.Quiet:
		return nil
	default:
		log.Errorf("unsupported report format: %v", a.LinterConfig.ReportType)
	}
	return nil
}

func CreateGitLabCommentsReport(ctx context.Context, gc *gitlab.Client, outputs map[string][]LinterOutput, lintername string, pid int, number int, logurl string) error {
	const comentDetailHeader = "<details>"
	const commentDetail = `
<br>If you have any questions about this comment, feel free to raise an issue here:

- **https://github.com/qiniu/reviewbot**

</details>`
	log.Infof("CreateMergeReviewDiscussion%v", outputs)
	var message string
	var errormessage string
	var tabletop10 string
	tabletop10 = "\n| FilePath    | Line    |ErrorMessage     |\n"
	tabletop10 += "| -------- | -------- | -------- |\n"
	var top10 = 0
	var totalerrorscount int
	totalerrorscount = 0
	// for combine the linter result
	if len(outputs) > 0 {
		for _, output := range outputs {
			totalerrorscount += len(output)
			for _, outputmessage := range output {
				errormessage = errormessage + "<br><code>" + outputmessage.File + ", line:" + strconv.Itoa(outputmessage.Line) + "</code> <br>&nbsp;" + outputmessage.Message
				if top10 < 10 {
					tabletop10 += "|" + outputmessage.File + "|" + strconv.Itoa(outputmessage.Line) + "|" + outputmessage.Message + "|\n"
				}
				top10++
			}
		}
		message = fmt.Sprintf("[**%s**]  check failed❌ , %v files exist errors,%v errors found.     This is [the detailed log](%s).<br>Top 10 errors as below：\r\n"+tabletop10+"\n%s", lintername, len(outputs), totalerrorscount, logurl, comentDetailHeader+errormessage+"<br>"+commentDetail)
	} else {
		message = fmt.Sprintf("[**%s**]  check passed✅ . \n%s", lintername, comentDetailHeader+commentDetail)
	}
	// create MR comments
	var optd gitlab.CreateMergeRequestNoteOptions
	var addedComments []*gitlab.Note
	optd.Body = &message
	err := RetryWithBackoff(ctx, func() error {
		log.Infof("CREATE NOTE %v,%v", pid, number)
		cm, resp, err := gc.Notes.CreateMergeRequestNote(pid, number, &optd)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusCreated {
			log.Errorf("create comment failed: %v", resp)
			return errCreateComment
		}
		addedComments = append(addedComments, cm)
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (g *GitlabProvider) IsRelated(file string, line int, startLine int) bool {
	return g.HunkChecker.InHunk(file, line, startLine)
}

func (g *GitlabProvider) GetFiles(predicate func(filepath string) bool) []string {
	var files []string
	for _, file := range g.MergeRequestChangedFiles {
		if predicate == nil || predicate(file.NewPath) {
			if file.NewPath == "" {
				continue
			}
			files = append(files, file.NewPath)
		}
	}
	return files
}

func MergeRequestSHA(gc *gitlab.Client, pid int, num int) (sf *DiffSha, err error) {
	var mr *gitlab.MergeRequest
	mr, _, err = gc.MergeRequests.GetMergeRequest(pid, num, nil)
	if err != nil {
		log.Errorf("get mr head sha failed:%v", err)
		return nil, errMrHeadSha
	}
	var diffsha = DiffSha{}
	diffsha.BaseSha = mr.DiffRefs.BaseSha
	diffsha.StartSha = mr.DiffRefs.StartSha
	diffsha.HeadSha = mr.DiffRefs.HeadSha
	return &diffsha, nil
}

func ListMergeRequestDiscussions(ctx context.Context, gc *gitlab.Client, number int, pid int) ([]*gitlab.Discussion, error) {
	opts := gitlab.ListMergeRequestDiscussionsOptions{
		PerPage: 200,
	}
	rd, re, er := gc.Discussions.ListMergeRequestDiscussions(pid, number, &opts, nil)
	if er != nil {
		log.Errorf("get mergerequest discussions failed:%v,response is:%v", er, re)
		return nil, er
	}
	return rd, nil
}

func DeleteMergeRequestDiscussions(ctx context.Context, gc *gitlab.Client, number int, pid int, dlist []*gitlab.Discussion, linterFlag string) error {
	for _, d := range dlist {
		if (d.Notes[0].Type == "DiffNote") && strings.HasPrefix(d.Notes[0].Body, linterFlag) {
			re, err := gc.Discussions.DeleteMergeRequestDiscussionNote(pid, number, "", d.Notes[0].ID, nil)
			if err != nil {
				log.Errorf("delete mergerequest discussions failed:%v,response is:%v", err, re)
				return err
			}
		}
	}
	return nil
}

func (g *GitlabProvider) ListMergeRequestsComments(ctx context.Context, gc *gitlab.Client, owner string, repo string, number int, pid int) ([]*gitlab.Note, error) {
	var allComments []*gitlab.Note
	opts := gitlab.ListMergeRequestNotesOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	err := RetryWithBackoff(ctx, func() error {
		comments, resp, err := gc.Notes.ListMergeRequestNotes(pid, number, &opts)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			log.Errorf("list mergerequest comments failed: %v", resp)
			return errListComment
		}
		allComments = comments
		return nil
	})
	return allComments, err
}

func CreateMergeReviewDiscussion(ctx context.Context, gc *gitlab.Client, owner string, repo string, number int, comments []*gitlab.CreateMergeRequestDiscussionOptions, pid int) ([]*gitlab.Discussion, error) {
	var addedComments []*gitlab.Discussion
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
			if resp.StatusCode != http.StatusCreated {
				log.Errorf("create mergerequest discussions failed: %v", resp)
				return errCreateComment
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

func (g *GitlabProvider) DeleteMergeReviewComments(ctx context.Context, gc *gitlab.Client, owner, repo string, comments []*gitlab.Note, pid int, number int) error {
	for _, comment := range comments {
		cmt := comment
		err := RetryWithBackoff(ctx, func() error {
			resp, err := gc.Notes.DeleteMergeRequestNote(pid, number, cmt.ID)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusNoContent {
				log.Errorf("delete mergerequest comments failed: %v", resp)

				return errDeleteComment
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
			resp, err := gc.Notes.DeleteMergeRequestNote(pid, number, cmt.ID, nil)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusNoContent {
				log.Errorf("delete mergerequest comments failed: %v", resp)
				return errDeleteComment
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

func constructMergeRequestDiscussion(linterOutputs map[string][]LinterOutput, linterName, commitID string, headSha string, baseSha string, startSha string) []*gitlab.CreateMergeRequestDiscussionOptions {
	var comments []*gitlab.CreateMergeRequestDiscussionOptions
	for z := range linterOutputs {
		for i := range linterOutputs[z] {
			var ptype = "text"
			message := fmt.Sprintf("%s %s\n%s",
				linterName, linterOutputs[z][i].Message, CommentFooter)
			if linterOutputs[z][i].StartLine != 0 {
				comments = append(comments, &gitlab.CreateMergeRequestDiscussionOptions{
					Body:     &message,
					CommitID: &commitID,
					Position: &gitlab.PositionOptions{
						NewPath:      &linterOutputs[z][i].File,
						NewLine:      &linterOutputs[z][i].Line,
						BaseSHA:      &baseSha,
						HeadSHA:      &headSha,
						StartSHA:     &startSha,
						PositionType: &ptype,
						OldPath:      &linterOutputs[z][i].File,
						OldLine:      &linterOutputs[z][i].Line,
					},
				})
			} else {
				comments = append(comments, &gitlab.CreateMergeRequestDiscussionOptions{
					Body: &message,

					Position: &gitlab.PositionOptions{
						NewPath:      &linterOutputs[z][i].File,
						BaseSHA:      &baseSha,
						HeadSHA:      &headSha,
						StartSHA:     &startSha,
						NewLine:      &linterOutputs[z][i].Line,
						PositionType: &ptype,
						OldPath:      &linterOutputs[z][i].File,
						OldLine:      &linterOutputs[z][i].Line,
					},
					CommitID: &commitID,
				})
			}
		}
	}
	return comments
}
