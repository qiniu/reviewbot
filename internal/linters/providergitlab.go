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
	"errors"
	"fmt"
	"golang.org/x/mod/semver"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/log"
	gitlab "github.com/xanzy/go-gitlab"
	gitv2 "sigs.k8s.io/prow/pkg/git/v2"
)

//var (
//	// ErrAfterTry is the error that the operation failed after 5 retries.
//	ErrAfterTry       = errors.New("failed after 5 retries")
//	ErrDeleteComment  = errors.New("delete comment failed")
//	ErrCreateComment  = errors.New("create comment failed")
//	ErrCreateCheckRun = errors.New("create check run failed")
//	ErrListComments   = errors.New("list comments failed")
//	ErrListCommits    = errors.New("list commits failed")
//)

// ListFiles lists all files for the specified pull request.
func getFloatGitLabVersion(ctx context.Context, gc *gitlab.Client) string {

	v, r, e := gc.Version.GetVersion()
	if e != nil {
		fmt.Errorf("failed to get gitlab verion: %v, response is:%v", e, r)
		return ""
	}
	return v.Version
}
func ListMergeRequestsFiles(ctx context.Context, gc *gitlab.Client, owner string, repo string, pid int, number int) ([]*gitlab.MergeRequestDiff, *gitlab.Response, error) {
	//fversion := getFloatGitLabVersion(ctx, gc)
	//if semver.Compare(fversion, "15.7") >= 0 {
	//	files, response, err := gc.MergeRequests.ListMergeRequestDiffs(pid, number, nil)
	//	if err != nil {
	//		return nil, nil, err
	//	}
	//	return files, response, err
	//}
	files, response, err := gc.MergeRequests.GetMergeRequestChanges(pid, number, nil)
	if err != nil {
		return nil, nil, err
	}
	//files
	return files.Changes, response, err
}

// FilterPullRequests filter full request by commit.
func FilterMergeRequestsWithCommit(ctx context.Context, gc *github.Client, owner, repo, headSha string) ([]*github.PullRequest, error) {
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

// RetryWithBackoff retries the function with backoff.
func RetryWithBackoffMerge(ctx context.Context, f func() error) error {
	log := lintersutil.FromContext(ctx)
	backoff := time.Second
	for i := 0; i < 5; i++ {
		err := f()
		if err == nil {
			return nil
		}

		if errors.Is(err, context.Canceled) {
			return err
		}

		log.Errorf("retrying after error: %v", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	return ErrAfterTry
}
func getPid() {

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

//func linterNamePrefix(linterName string) string {
//	return fmt.Sprintf("[%s]", linterName)
//}

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

//const Reference = "If you have any questions about this comment, feel free to [raise an issue here](https://github.com/qiniu/reviewbot)."

func toGitlabCheckRunAnnotations(linterOutputs map[string][]LinterOutput) []*github.CheckRunAnnotation {
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
	//TODO implement me
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

	//gc.MergeRequests.GetMergeRequestDiffVersions()
	cmt := comment
	var opt gitlab.CreateMergeRequestNoteOptions
	opt.Body = &cmt.Body

	cm, resp, err := g.GitLabClient.Notes.CreateMergeRequestNote(g.MergeRequestEvent.ObjectAttributes.TargetProjectID, number, &opt)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create comment failed: %v", resp)
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
	//TODO implement me

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
func ReportFormMartCheck(gc *gitlab.Client, reportFomart config.GitlabReportType) (string config.GitlabReportType) {
	v, r, e := gc.Version.GetVersion()
	if e != nil {
		log.Fatalf("Failed to get version: %v,response is %v", e, r)
		return config.QuietGitlab
	}
	if semver.Compare(v.Version, "10.8") < 0 {
		return config.GitlabComment
	}
	return reportFomart

}

func (g *GitlabProvider) HandleComments(ctx context.Context, outputs map[string][]LinterOutput) error {
	return nil
}

func (g *GitlabProvider) Report(ctx context.Context, a Agent, lintResults map[string][]LinterOutput) error {
	linterName := a.LinterConfig.Name
	org := a.Provider.GetCodeReviewInfo().Org
	repo := a.Provider.GetCodeReviewInfo().Repo
	num := a.Provider.GetCodeReviewInfo().Number
	orgRepo := fmt.Sprintf("%s/%s", org, repo)
	reportformat := ReportFormMartCheck(g.GitLabClient, a.LinterConfig.GitlabReportFormat)
	switch reportformat {
	case config.GitlabCommentAndDiscussion:
		var pid = g.MergeRequestEvent.ObjectAttributes.TargetProjectID
		existedComments, err := g.ListMergeRequestsComments(context.Background(), g.GitLabClient, org, repo, num, pid)
		if err != nil {
			log.Errorf("failed to list comments: %v", err)
			return err
		}

		// filter out the comments that are not related to the linter
		var existedCommentsToKeep []*gitlab.Note
		linterFlag := linterNamePrefix(linterName)
		for _, comment := range existedComments {
			if strings.HasPrefix(comment.Body, linterFlag) {
				existedCommentsToKeep = append(existedCommentsToKeep, comment)
			}
		}
		log.Infof("%s found %d existed comments for this PR %d (%s) \n", linterFlag, len(existedCommentsToKeep), num, orgRepo)

		toAdds, toDeletes := filterLinterOutputsForGitLab(lintResults, existedCommentsToKeep)
		if err := DeleteMergeReviewCommentsForGitLab(context.Background(), g.GitLabClient, org, repo, toDeletes, pid, num); err != nil {
			log.Errorf("failed to delete comments: %v", err)
			return err
		}
		log.Infof("%s delete %d comments for this PR %d (%s) \n", linterFlag, len(toDeletes), num, orgRepo)
		h, b, s, e := MergeRequestSHA(g.GitLabClient, pid, num)
		if e != nil {
			log.Errorf("failed to delete comments: %v", e)
			return e
		}

		logURL := a.GenLogViewUrl()
		commerr := CreateGitLabCommnetsReport(context.Background(), g.GitLabClient, toAdds, linterName, pid, num, logURL)
		if commerr != nil {
			log.Errorf("failed to delete comments: %v", commerr)
		}

		discussion := constructMergeRequestDiscussion(toAdds, linterFlag, g.MergeRequestEvent.ObjectAttributes.LastCommit.ID, h, b, s)
		if len(discussion) == 0 {
			return nil
		}
		log.Infof("discussion%v", discussion)

		// Add the comments
		addedDis, err := CreateMergeReviewDiscussion(context.Background(), g.GitLabClient, org, repo, num, discussion, pid)
		if err != nil {
			log.Errorf("failed to post comments: %v", err)
			return err
		}
		log.Infof("[%s] add %d comments for this PR %d (%s) \n", linterName, len(addedDis), num, orgRepo)
		//metric.NotifyWebhookByText(ConstructGotchaMsg(linterName, g.MergeRequestEvent.Project.WebURL, toAdds[][0].Message, lintResults))
	case config.GitlabComment:
		var pid = g.MergeRequestEvent.ObjectAttributes.TargetProjectID
		existedComments, err := g.ListMergeRequestsComments(context.Background(), g.GitLabClient, org, repo, num, pid)
		if err != nil {
			log.Errorf("failed to list comments: %v", err)
			return err
		}

		// filter out the comments that are not related to the linter
		var existedCommentsToKeep []*gitlab.Note
		linterFlag := linterNamePrefix(linterName)
		for _, comment := range existedComments {
			if strings.HasPrefix(comment.Body, linterFlag) {
				existedCommentsToKeep = append(existedCommentsToKeep, comment)
			}
		}
		log.Infof("%s found %d existed comments for this PR %d (%s) \n", linterFlag, len(existedCommentsToKeep), num, orgRepo)

		toAdds, toDeletes := filterLinterOutputsForGitLab(lintResults, existedCommentsToKeep)
		if err := DeleteMergeReviewCommentsForGitLab(context.Background(), g.GitLabClient, org, repo, toDeletes, pid, num); err != nil {
			log.Errorf("failed to delete comments: %v", err)
			return err
		}
		log.Infof("%s delete %d comments for this PR %d (%s) \n", linterFlag, len(toDeletes), num, orgRepo)

		logURL := a.GenLogViewUrl()
		commerr := CreateGitLabCommnetsReport(context.Background(), g.GitLabClient, toAdds, linterName, pid, num, logURL)
		if commerr != nil {
			log.Errorf("failed to delete comments: %v", commerr)
		}

	case config.QuietGitlab:
		return nil
	default:
		log.Errorf("unsupported report format: %v", a.LinterConfig.GitlabReportFormat)
	}

	return nil
}
func CreateGitLabDisscussionReport(ctx context.Context, gc gitlab.Client, outputs map[string][]LinterOutput, pid int, number int, lintername string) {

}
func CreateGitLabCommnetsReport(ctx context.Context, gc *gitlab.Client, outputs map[string][]LinterOutput, lintername string, pid int, number int, logurl string) error {

	const ComentDetalHeader = "<details>"
	const CommentDetal = `
<br>If you have any questions about this comment, feel free to raise an issue here:

- **https://github.com/qiniu/reviewbot**

</details>`
	log.Infof("CreateMergeReviewDiscussion%v", outputs)
	var message string
	var errormessage string
	var totalerrorscount int
	totalerrorscount = 0

	if len(outputs) > 0 {
		for _, output := range outputs {
			totalerrorscount = totalerrorscount + len(output)
			for _, outputmesage := range output {
				errormessage = errormessage + "<br>" + outputmesage.File + ", line:" + strconv.Itoa(outputmesage.Line) + ", " + outputmesage.Message
			}
		}

		message = fmt.Sprintf("[**%s**]  check failed❌ , %v files exist errors,%v errors found.     This is [the detailed log](%s).\n%s", lintername, len(outputs), totalerrorscount, logurl, ComentDetalHeader+errormessage+"<br>"+CommentDetal)
	} else {
		message = fmt.Sprintf("[**%s**]  check passed✅ . \n%s", lintername, ComentDetalHeader+CommentDetal)
	}

	var optd gitlab.CreateMergeRequestDiscussionOptions
	var addedComments []*gitlab.Discussion

	optd.Body = &message
	err := RetryWithBackoff(ctx, func() error {
		log.Infof("CREATE NOTE %v,%v", pid, number)

		cm, resp, err := gc.Discussions.CreateMergeRequestDiscussion(pid, number, &optd)
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
func MergeRequestSHA(gc *gitlab.Client, pid int, num int) (HeadSha string, BaseSha string, StartSha string, err error) {
	var mr *gitlab.MergeRequest
	mr, _, err = gc.MergeRequests.GetMergeRequest(pid, num, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("get mr head sha failed: %v", err)

	}

	return mr.DiffRefs.HeadSha, mr.DiffRefs.BaseSha, mr.DiffRefs.StartSha, nil
}
func constructMergeRequestComments(linterOutputs map[string][]LinterOutput, linterName, commitID string, headSha string, baseSha string, startSha string) []*gitlab.Note {
	var comments []*gitlab.Note
	for file, outputs := range linterOutputs {
		for _, output := range outputs {
			message := fmt.Sprintf("%s %s\n%s", linterName, output.Message, CommentFooter)
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

func (g *GitlabProvider) ListMergeRequestsComments(ctx context.Context, gc *gitlab.Client, owner string, repo string, number int, pid int) ([]*gitlab.Note, error) {
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
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("list comments failed: %v", resp)
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
func (g *GitlabProvider) DeleteMergeReviewComments(ctx context.Context, gc *gitlab.Client, owner, repo string, comments []*gitlab.Note, pid int, number int) error {
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

func (g *GitlabProvider) MergeRequestSHA(ctx context.Context, gc *gitlab.Client, pid int, num int) (HeadSha string, BaseSha string, StartSha string, err error) {
	var mr *gitlab.MergeRequest
	mr, _, err = gc.MergeRequests.GetMergeRequest(pid, num, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("get mr head sha failed: %v", err)

	}

	return mr.DiffRefs.HeadSha, mr.DiffRefs.BaseSha, mr.DiffRefs.StartSha, nil
}

func CreateMergeReviewCommentsTotal(ctx context.Context, gc *gitlab.Client, owner string, repo string, number int, comments []*gitlab.Note, pid int) {

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
			if resp.StatusCode != http.StatusCreated {
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
func (g *GitlabProvider) CreateMergeReviewDiscussion(ctx context.Context, gc *gitlab.Client, owner string, repo string, number int, comments []*gitlab.CreateMergeRequestDiscussionOptions, pid int) ([]*gitlab.Discussion, error) {
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

// CreateGithubChecks creates github checks for the specified pull request.
