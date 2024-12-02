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
	"net/http"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/cache"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/reviewbot/internal/metric"
	"github.com/qiniu/x/log"
)

var (
	// ErrAfterTry is the error that the operation failed after 5 retries.
	ErrAfterTry                = errors.New("failed after 5 retries")
	ErrDeleteComment           = errors.New("delete comment failed")
	ErrCreateComment           = errors.New("create comment failed")
	ErrCreateCheckRun          = errors.New("create check run failed")
	ErrListComments            = errors.New("list comments failed")
	ErrListCommits             = errors.New("list commits failed")
	ErrUnexpectedTransportType = errors.New("unexpected transport type")
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

// RetryWithBackoff retries the function with backoff.
func RetryWithBackoff(ctx context.Context, f func() error) error {
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

func linterNamePrefix(linterName string) string {
	return fmt.Sprintf("**[%s]** <sub>reported by [reviewbot](https://github.com/qiniu/reviewbot):cow:</sub>\n", linterName)
}

func constructPullRequestComments(linterOutputs map[string][]LinterOutput, linterName, commitID string) []*github.PullRequestComment {
	var comments []*github.PullRequestComment
	for file, outputs := range linterOutputs {
		for _, output := range outputs {
			// use the typed message as first priority
			var message string
			if output.TypedMessage != "" {
				message = fmt.Sprintf("%s %s",
					linterName, output.TypedMessage)
			} else {
				message = fmt.Sprintf("%s %s",
					linterName, output.Message)
			}

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

// make sure the GithubProvider implements the Provider interface.
var _ Provider = (*GithubProvider)(nil)

type GithubProvider struct {
	// GitHubClient is the GitHub client.
	GithubClient *github.Client
	// HunkChecker is the hunk checker for the file.
	HunkChecker *FileHunkChecker

	// PullRequestChangedFiles is the changed files of a pull request.
	PullRequestChangedFiles []*github.CommitFile
	// PullRequestEvent is the event of a pull request.
	PullRequestEvent github.PullRequestEvent
	// ProviderInfo is the provider information.
	ProviderInfo ProviderInfo
}

func NewGithubProvider(ctx context.Context, githubClient *github.Client, pullRequestEvent github.PullRequestEvent, options ...GithubProviderOption) (*GithubProvider, error) {
	p := &GithubProvider{
		GithubClient:     githubClient,
		PullRequestEvent: pullRequestEvent,
	}

	for _, option := range options {
		option(p)
	}

	if p.PullRequestChangedFiles == nil {
		files, resp, err := ListPullRequestsFiles(ctx, p.GithubClient, pullRequestEvent.GetRepo().GetOwner().GetLogin(), pullRequestEvent.GetRepo().GetName(), pullRequestEvent.GetPullRequest().GetNumber())
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			log.Errorf("failed to list pull request files, status code: %d", resp.StatusCode)
			return nil, errListFile
		}
		p.PullRequestChangedFiles = files
	}

	if p.HunkChecker == nil {
		checker, err := newGithubHunkChecker(p.PullRequestChangedFiles)
		if err != nil {
			return nil, err
		}
		p.HunkChecker = checker
	}

	return p, nil
}

// GithubProviderOption allows customizing the provider creation.
type GithubProviderOption func(*GithubProvider)

// WithPullRequestChangedFiles sets the pull request changed files for the provider.
func WithPullRequestChangedFiles(files []*github.CommitFile) GithubProviderOption {
	return func(p *GithubProvider) {
		p.PullRequestChangedFiles = files
	}
}

// WithHunkChecker sets the hunk checker for the provider.
func WithHunkChecker(checker *FileHunkChecker) GithubProviderOption {
	return func(p *GithubProvider) {
		p.HunkChecker = checker
	}
}

// WithGitHubProviderInfo sets the provider information for the provider.
func WithGitHubProviderInfo(info ProviderInfo) GithubProviderOption {
	return func(p *GithubProvider) {
		p.ProviderInfo = info
	}
}

func (g *GithubProvider) HandleComments(ctx context.Context, outputs map[string][]LinterOutput) error {
	return nil
}

func (g *GithubProvider) Report(ctx context.Context, a Agent, lintResults map[string][]LinterOutput) error {
	log := lintersutil.FromContext(ctx)
	linterName := a.LinterConfig.Name
	org := a.Provider.GetCodeReviewInfo().Org
	repo := a.Provider.GetCodeReviewInfo().Repo
	num := a.Provider.GetCodeReviewInfo().Number
	orgRepo := fmt.Sprintf("%s/%s", org, repo)

	switch a.LinterConfig.ReportType {
	case config.GitHubCheckRuns:
		check := newBaseCheckRun(a, lintResults)
		ch, err := g.CreateCheckRun(ctx, org, repo, check)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Errorf("failed to create github checks: %v", err)
			}
			return err
		}
		log.Infof("[%s] create check run success, HTML_URL: %v", linterName, ch.GetHTMLURL())

		metric.NotifyWebhookByText(ConstructGotchaMsg(linterName, a.Provider.GetCodeReviewInfo().URL, ch.GetHTMLURL(), lintResults))
	case config.GitHubPRReview:
		comments, err := g.ProcessComments(ctx, a, lintResults)
		if err != nil {
			log.Errorf("failed to process need to add comments: %v", err)
			return err
		}
		if len(comments) == 0 {
			log.Infof("[%s] no comments need to add", linterName)
			return nil
		}
		// Add the comments
		addedCmts, err := g.CreatePullReviewComments(ctx, org, repo, num, comments)
		if err != nil {
			log.Errorf("failed to post comments: %v", err)
			return err
		}
		log.Infof("[%s] add %d comments for this PR %d (%s) \n", linterName, len(addedCmts), num, orgRepo)
		metric.NotifyWebhookByText(ConstructGotchaMsg(linterName, a.Provider.GetCodeReviewInfo().URL, addedCmts[0].GetHTMLURL(), lintResults))
	case config.GitHubMixType:
		// report all lint results as a check run summary, but not annotations
		check := newMixCheckRun(a, lintResults)
		ch, err := g.CreateCheckRun(ctx, org, repo, check)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Errorf("failed to create github checks: %v", err)
			}
			return err
		}
		log.Infof("[%s] create check run success, HTML_URL: %v", linterName, ch.GetHTMLURL())

		// report top 10 lint results to pull request review comments at most
		top10LintResults := listTop10LintResults(lintResults)
		comments, err := g.ProcessComments(ctx, a, top10LintResults)
		if err != nil {
			log.Errorf("failed to process need to add comments: %v", err)
			return err
		}
		if len(comments) == 0 {
			log.Infof("[%s] no comments need to add", linterName)
			return nil
		}

		addedCmts, err := g.CreatePullReviewComments(ctx, org, repo, num, comments)
		if err != nil {
			log.Errorf("failed to post comments: %v", err)
			return err
		}
		log.Infof("[%s] add %d comments for this PR %d (%s) \n", linterName, len(addedCmts), num, orgRepo)

		metric.NotifyWebhookByText(ConstructGotchaMsg(linterName, a.Provider.GetCodeReviewInfo().URL, ch.GetHTMLURL(), lintResults))
	case config.Quiet:
		return nil
	default:
		log.Errorf("unsupported report format: %v", a.LinterConfig.ReportType)
	}

	return nil
}

// lists the top 10 lint results.
func listTop10LintResults(lintResults map[string][]LinterOutput) map[string][]LinterOutput {
	seenMsgs := make(map[string]struct{})
	seenLinterOutput := make(map[string]struct{})
	result := make(map[string][]LinterOutput)
	count := 0

	// Filter out different outputs according to output.Message, when the results are greater than or equal to 10, return it directly.
	for file, outputs := range lintResults {
		for _, output := range outputs {
			if count >= 10 {
				return result
			}
			if _, exists := seenMsgs[output.Message]; !exists {
				seenMsgs[output.Message] = struct{}{}
				seenLinterOutput[fmt.Sprintf("%s:%d", output.File, output.Line)] = struct{}{}
				result[file] = append(result[file], output)
				count++
			}
		}
	}

	// After the above processing, if the results are less than 10, then add the output according to the file and line.
	for file, outputs := range lintResults {
		for _, output := range outputs {
			if count >= 10 {
				return result
			}
			key := fmt.Sprintf("%s:%d", output.File, output.Line)
			if _, exists := seenLinterOutput[key]; !exists {
				result[file] = append(result[file], output)
				count++
			}
		}
	}

	return result
}

func (g *GithubProvider) IsRelated(file string, line int, startLine int) bool {
	return g.HunkChecker.InHunk(file, line, startLine)
}

func (g *GithubProvider) GetFiles(predicate func(filepath string) bool) []string {
	var files []string
	for _, file := range g.PullRequestChangedFiles {
		if predicate == nil || predicate(file.GetFilename()) {
			if file.GetStatus() == "removed" {
				continue
			}
			files = append(files, file.GetFilename())
		}
	}
	return files
}

// ListPullRequestsComments lists all comments on the specified pull request.
func (g *GithubProvider) ListPullRequestsComments(ctx context.Context, owner string, repo string, number int) ([]*github.PullRequestComment, error) {
	var allComments []*github.PullRequestComment
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	err := RetryWithBackoff(ctx, func() error {
		comments, resp, err := g.GithubClient.PullRequests.ListComments(ctx, owner, repo, number, opts)
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

// DeletePullReviewComments deletes the specified comments on the pull request.
func (g *GithubProvider) DeletePullReviewComments(ctx context.Context, owner, repo string, comments []*github.PullRequestComment) error {
	log := lintersutil.FromContext(ctx)
	for _, comment := range comments {
		cmt := comment
		err := RetryWithBackoff(ctx, func() error {
			resp, err := g.GithubClient.PullRequests.DeleteComment(ctx, owner, repo, cmt.GetID())
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusNoContent {
				log.Errorf("delete comment failed: %v", resp)
				return ErrDeleteComment
			}
			return nil
		})
		if err != nil {
			return err
		}

		log.Infof("delete comment success: %v", comment.GetHTMLURL())
	}

	return nil
}

// CreatePullReviewComments creates the specified comments on the pull request.
func (g *GithubProvider) CreatePullReviewComments(ctx context.Context, owner string, repo string, number int, comments []*github.PullRequestComment) ([]*github.PullRequestComment, error) {
	log := lintersutil.FromContext(ctx)
	var addedComments []*github.PullRequestComment
	for _, comment := range comments {
		cmt := comment
		err := RetryWithBackoff(ctx, func() error {
			cm, resp, err := g.GithubClient.PullRequests.CreateComment(ctx, owner, repo, number, cmt)
			if err != nil {
				log.Errorf("create comment failed: %v", err)
				return err
			}
			if resp.StatusCode != http.StatusCreated {
				log.Errorf("create comment failed: %v", resp)
				return ErrCreateComment
			}
			log.Infof("create comment success: %v", cm.GetHTMLURL())
			addedComments = append(addedComments, cm)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return addedComments, nil
}

// CreateCheckRun submits the check run to GitHub.
func (g *GithubProvider) CreateCheckRun(ctx context.Context, owner string, repo string, check github.CreateCheckRunOptions) (*github.CheckRun, error) {
	var ch *github.CheckRun
	err := RetryWithBackoff(ctx, func() error {
		checkRun, resp, err := g.GithubClient.Checks.CreateCheckRun(ctx, owner, repo, check)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Errorf("create check run failed: %v, check: %v", err, check)
			}
			return err
		}

		if resp.StatusCode != http.StatusCreated {
			log.Errorf("unexpected response when create check run: %v", resp)
			return ErrCreateCheckRun
		}

		ch = checkRun
		return nil
	})

	return ch, err
}

func (g *GithubProvider) ListCommits(ctx context.Context, org, repo string, number int) ([]Commit, error) {
	log := lintersutil.FromContext(ctx)
	opts := &github.ListOptions{
		PerPage: 100,
	}
	var allCommits []Commit

	for {
		commits, resp, err := g.GithubClient.PullRequests.ListCommits(ctx, org, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("listing commits: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Errorf("list commits failed: %v", resp)
			return nil, ErrListCommits
		}

		for _, commit := range commits {
			allCommits = append(allCommits, Commit{
				Message: commit.GetCommit().GetMessage(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allCommits, nil
}

func (g *GithubProvider) ListComments(ctx context.Context, owner string, repo string, number int) ([]Comment, error) {
	comments, resp, err := g.GithubClient.Issues.ListComments(ctx, owner, repo, number, &github.IssueListCommentsOptions{})
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
			ID:        comment.GetID(),
			Body:      comment.GetBody(),
			CreatedAt: comment.GetCreatedAt().Time,
			UpdatedAt: comment.GetUpdatedAt().Time,
			URL:       comment.GetURL(),
			HTMLURL:   comment.GetHTMLURL(),
			IssueURL:  comment.GetIssueURL(),
		})
	}

	return allComments, nil
}

func (g *GithubProvider) DeleteComment(ctx context.Context, owner string, repo string, commentID int64) error {
	_, err := g.GithubClient.Issues.DeleteComment(ctx, owner, repo, commentID)
	return err
}

func (g *GithubProvider) CreateComment(ctx context.Context, owner string, repo string, number int, comment *Comment) (*Comment, error) {
	log := lintersutil.FromContext(ctx)
	c, resp, err := g.GithubClient.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
		Body: &comment.Body,
	})

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		log.Errorf("create comment failed: %v", resp)
		return nil, ErrCreateComment
	}

	return &Comment{
		ID:        c.GetID(),
		Body:      c.GetBody(),
		CreatedAt: c.GetCreatedAt().Time,
		UpdatedAt: c.GetUpdatedAt().Time,
		URL:       c.GetURL(),
		HTMLURL:   c.GetHTMLURL(),
		IssueURL:  c.GetIssueURL(),
	}, nil
}

func (g *GithubProvider) GetCodeReviewInfo() CodeReview {
	return CodeReview{
		Org:       g.PullRequestEvent.Repo.GetOwner().GetLogin(),
		Repo:      g.PullRequestEvent.Repo.GetName(),
		Number:    g.PullRequestEvent.GetPullRequest().GetNumber(),
		Author:    g.PullRequestEvent.GetPullRequest().GetUser().GetLogin(),
		URL:       g.PullRequestEvent.GetPullRequest().GetHTMLURL(),
		HeadSHA:   g.PullRequestEvent.GetPullRequest().GetHead().GetSHA(),
		UpdatedAt: g.PullRequestEvent.GetPullRequest().GetUpdatedAt().Time,
	}
}

func (g *GithubProvider) ProcessComments(ctx context.Context, a Agent, lintResults map[string][]LinterOutput) ([]*github.PullRequestComment, error) {
	org := a.Provider.GetCodeReviewInfo().Org
	repo := a.Provider.GetCodeReviewInfo().Repo
	num := a.Provider.GetCodeReviewInfo().Number
	linterName := a.LinterConfig.Name
	log := lintersutil.FromContext(ctx)
	orgRepo := org + "/" + repo

	// List existing comments
	existedComments, err := g.ListPullRequestsComments(ctx, org, repo, num)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Errorf("failed to list comments: %v", err)
		}
		return nil, err
	}

	// filter out the comments that are not related to the linter
	var existedCommentsToKeep []*github.PullRequestComment
	linterFlag := linterNamePrefix(linterName)
	for _, comment := range existedComments {
		if strings.HasPrefix(comment.GetBody(), linterFlag) {
			existedCommentsToKeep = append(existedCommentsToKeep, comment)
		}
	}
	log.Infof("[%s] found %d existed comments for this PR %d (%s) \n", linterName, len(existedCommentsToKeep), num, orgRepo)

	toAdds, toDeletes := filterLinterOutputs(lintResults, existedCommentsToKeep)
	if err := g.DeletePullReviewComments(ctx, org, repo, toDeletes); err != nil {
		log.Errorf("failed to delete comments: %v", err)
		return nil, err
	}
	log.Infof("[%s] delete %d comments for this PR %d (%s) \n", linterName, len(toDeletes), num, orgRepo)
	comments := constructPullRequestComments(toAdds, linterNamePrefix(linterName), a.Provider.GetCodeReviewInfo().HeadSHA)
	return comments, nil
}

func (g *GithubProvider) GetToken() (string, error) {
	// with platform and org for uniqueness
	key := fmt.Sprintf("%s:%s", config.GitHub, g.PullRequestEvent.Repo.GetOwner().GetLogin())
	token, ok := cache.DefaultTokenCache.GetToken(key)
	if ok {
		return token, nil
	}

	token, err := g.refreshToken()
	if err != nil {
		return "", err
	}

	// set the token with a little less than 1 hour expiration since github app token will expire in 1 hours
	// see https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation#about-installation-access-tokens
	exp := time.Now().Add(time.Hour - time.Minute*5)
	cache.DefaultTokenCache.SetToken(key, token, exp)
	log.Infof("set refreshed token for %s, key: %s, exp: %s", token, key, exp.Format(time.RFC3339))
	return token, nil
}

func (g *GithubProvider) GetProviderInfo() ProviderInfo {
	return g.ProviderInfo
}

// refreshToken refresh the GitHub App token.
func (g *GithubProvider) refreshToken() (string, error) {
	tr, ok := g.GithubClient.Client().Transport.(*ghinstallation.Transport)
	if !ok {
		log.Errorf("unexpected transport type: %T", g.GithubClient.Client().Transport)
		return "", ErrUnexpectedTransportType
	}
	token, err := tr.Token(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get installation token: %w", err)
	}
	return token, nil
}

// newBaseCheckRun creates the base check run options.
func newBaseCheckRun(a Agent, lintErrs map[string][]LinterOutput) github.CreateCheckRunOptions {
	var (
		headSha    = a.Provider.GetCodeReviewInfo().HeadSHA
		startTime  = a.Provider.GetCodeReviewInfo().UpdatedAt
		linterName = a.LinterConfig.Name
	)

	annotations := toGithubCheckRunAnnotations(lintErrs)
	if len(annotations) > 50 {
		annotations = annotations[:50]
	}

	check := github.CreateCheckRunOptions{
		Name:    linterName,
		HeadSHA: headSha,
		Status:  github.String("completed"),
		StartedAt: &github.Timestamp{
			Time: startTime,
		},
		CompletedAt: &github.Timestamp{
			Time: time.Now(),
		},
		Output: &github.CheckRunOutput{
			Title:       github.String(fmt.Sprintf("%s found %d issues related to your changes", linterName, len(annotations))),
			Annotations: annotations,
		},
	}

	logURL := a.GenLogViewURL()
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

	return check
}

func newMixCheckRun(a Agent, lintErrs map[string][]LinterOutput) github.CreateCheckRunOptions {
	check := newBaseCheckRun(a, lintErrs)
	if len(lintErrs) == 0 {
		// if no lint errors, just return the base check run
		return check
	}

	// delete annotations since it will use PR review comments to show the details in mix report type
	check.Output.Annotations = nil

	// but still use details to all linter outputs
	var b strings.Builder
	// Add title and description
	b.WriteString("## 🔍 Check Results Details\n\n")
	b.WriteString("The following shows all issues found related to your changes:\n\n")

	// Group results by file
	b.WriteString("```text\n")
	for file, outputs := range lintErrs {
		for _, output := range outputs {
			b.WriteString(fmt.Sprintf("%s:%d: %s\n", file, output.Line, output.Message))
		}
	}
	b.WriteString("```\n")

	// Add action guide
	b.WriteString("\n### 🔄 How to Handle?\n\n")
	b.WriteString("1. Fix the issues above and submit your code again\n")
	b.WriteString("2. Or click the `Re-run` button to run the check again\n")
	b.WriteString("3. If you think this is a false positive, please contact your support team\n\n")

	// Add notes
	b.WriteString("### ℹ️ Notes\n\n")
	b.WriteString("- To avoid too many comments, only the top 10 issues will be shown in PR comments\n")
	b.WriteString("- For any other issues, feel free to create an issue in [reviewbot](https://github.com/qiniu/reviewbot) repository\n")

	check.Output.Text = github.String(b.String())
	return check
}

func newGithubHunkChecker(commitFiles []*github.CommitFile) (*FileHunkChecker, error) {
	hunks := make(map[string][]Hunk)
	for _, commitFile := range commitFiles {
		if !isValidGithubCommitFile(commitFile) {
			continue
		}

		fileHunks, err := parseGithubPatch(commitFile.GetPatch())
		if err != nil {
			return nil, err
		}

		if existing, ok := hunks[commitFile.GetFilename()]; ok {
			log.Warnf("duplicate commitFiles: %v, %v", commitFile, existing)
			continue
		}

		hunks[commitFile.GetFilename()] = fileHunks
	}

	return NewFileHunkChecker(hunks), nil
}

func isValidGithubCommitFile(file *github.CommitFile) bool {
	return file != nil && file.GetPatch() != "" && file.GetStatus() != "removed"
}

func parseGithubPatch(patch string) ([]Hunk, error) {
	return ParsePatch(patch)
}
