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
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/reviewbot/internal/metric"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
	gitv2 "sigs.k8s.io/prow/pkg/git/v2"
)

var (
	pullRequestHandlers = map[string]PullRequestHandlerFunc{}
	linterLanguages     = map[string][]string{}
)

const artifactDirFlag = "reviewbot"

// PullRequestHandlerFunc knows how to handle a pull request event.
type PullRequestHandlerFunc func(*xlog.Logger, Agent) error

// RegisterPullRequestHandler registers a PullRequestHandlerFunc for the given linter name.
func RegisterPullRequestHandler(name string, handler PullRequestHandlerFunc) {
	pullRequestHandlers[name] = handler
}

// RegisterLinterLanguages registers the languages supported by the linter.
func RegisterLinterLanguages(name string, languages []string) {
	linterLanguages[name] = languages
}

// PullRequestHandler returns a PullRequestHandlerFunc for the given linter name.
func PullRequestHandler(name string) PullRequestHandlerFunc {
	if handler, ok := pullRequestHandlers[name]; ok {
		return handler
	}
	return nil
}

// TotalPullRequestHandlers returns all registered PullRequestHandlerFunc.
func TotalPullRequestHandlers() map[string]PullRequestHandlerFunc {
	var handlers = make(map[string]PullRequestHandlerFunc, len(pullRequestHandlers))
	for name, handler := range pullRequestHandlers {
		handlers[name] = handler
	}

	return handlers
}

// LinterLanguages returns the languages supported by the linter.
func Languages(linterName string) []string {
	return linterLanguages[linterName]
}

// Linter knows how to execute linters.
type Linter interface {
	// Run executes a linter command.
	Run(log *xlog.Logger, args ...string) ([]byte, error)
	// Parse parses the output of a linter command.
	Parse(log *xlog.Logger, output []byte) (map[string][]LinterOutput, error)
}

type LinterOutput struct {
	// File is the File name
	File string
	// Line is the Line number
	Line int
	// Column is the Column number
	Column int
	// Message is the staticcheck Message
	Message string
	//StartLine required when using multi-line comments
	StartLine int
}

// Agent knows necessary information in order to run linters.
type Agent struct {
	// GitHubClient is the GitHub client.
	GithubClient *github.Client
	// GitClient is the Git client factory.
	GitClient gitv2.ClientFactory
	// LinterConfig is the linter configuration.
	LinterConfig config.Linter
	// PullRequestEvent is the event of a pull request.
	PullRequestEvent github.PullRequestEvent
	// PullRequestChangedFiles is the changed files of a pull request.
	PullRequestChangedFiles []*github.CommitFile
	// LinterName is the linter name.
	LinterName string
	// RepoDir is the repo directory.
	RepoDir string
}

const CommentFooter = `
<details>

If you have any questions about this comment, feel free to raise an issue here:

- **https://github.com/qiniu/reviewbot**

</details>`

// ------------------------ Linter ------------------------

// LinterParser is a function that parses the output of a linter command.
// It returns the structured lint results if parsed successfully and unexpected lines if failed.
// The unexpected lines are the lines that cannot be parsed.
type LinterParser func(*xlog.Logger, []byte) (map[string][]LinterOutput, []string)

func GeneralHandler(log *xlog.Logger, a Agent, linterParser func(*xlog.Logger, []byte) (map[string][]LinterOutput, []string)) error {
	linterName := a.LinterName
	output, err := ExecRun(a)
	if err != nil {
		// NOTE(CarlJi): the error is *ExitError, it seems to have little information and needs to be handled in a better way.
		log.Warnf("%s run with exit code: %v, mark and continue", linterName, err)
	}

	// even if the output is empty, we still need to parse it
	// since we need delete the existed comments related to the linter

	lintResults, unexpected := linterParser(log, output)
	if len(unexpected) > 0 {
		msg := lintersutil.LimitJoin(unexpected, 1000)
		// just log the unexpected lines and notify the webhook, no need to return error
		log.Warnf("unexpected lines: %v", msg)
		metric.NotifyWebhookByText(ConstructUnknownMsg(linterName, a.PullRequestEvent.Repo.GetFullName(), a.PullRequestEvent.PullRequest.GetHTMLURL(), log.ReqId, msg))
	}

	return Report(log, a, lintResults)
}

// ExecRun executes a command.
func ExecRun(a Agent) ([]byte, error) {
	command := a.LinterConfig.Command
	executable := command[0]
	var cmdArgs []string
	if len(command) > 1 {
		cmdArgs = command[1:]
	}
	cmdArgs = append(cmdArgs, a.LinterConfig.Args...)
	c := exec.Command(executable, cmdArgs...)
	c.Dir = a.LinterConfig.WorkDir

	// create a temp dir for the artifact
	artifact, err := os.MkdirTemp("", artifactDirFlag)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(artifact)
	c.Env = append(os.Environ(), fmt.Sprintf("ARTIFACT=%s", artifact))
	c.Env = append(c.Env, a.LinterConfig.Env...)

	log.Infof("run command: %v, workdir: %v", c, c.Dir)
	output, execErr := c.CombinedOutput()

	// read all files under the artifact dir
	var fileContent []byte
	artifactFiles, err := os.ReadDir(artifact)
	if err != nil {
		return nil, err
	}

	var idx int
	for _, file := range artifactFiles {
		if file.IsDir() {
			continue
		}
		log.Infof("artifact file: %v", file.Name())
		content, err := os.ReadFile(fmt.Sprintf("%s/%s", artifact, file.Name()))
		if err != nil {
			return nil, err
		}
		if len(content) == 0 {
			continue
		}
		if idx > 0 {
			fileContent = append(fileContent, '\n')
		}
		fileContent = append(fileContent, content...)
		idx++
	}

	// use the content of the files under Artifact dir as first priority
	if len(fileContent) > 0 {
		log.Debugf("artifact files used instead. legacy output:\n%v, now:\n%v", string(output), string(fileContent))
		output = fileContent
	}

	return output, execErr
}

// GeneralParse parses the output of a linter command.
func GeneralParse(log *xlog.Logger, output []byte) (map[string][]LinterOutput, []string) {
	return Parse(log, output, GeneralLineParser)
}

// Report reports the lint results.
// This function should be always called even in custom linter handler since it will filter out the lint errors that are not related to the PR.
// and handle some special cases like auto-generated files.
func Report(log *xlog.Logger, a Agent, lintResults map[string][]LinterOutput) error {
	var (
		num        = a.PullRequestEvent.GetNumber()
		org        = a.PullRequestEvent.Repo.GetOwner().GetLogin()
		repo       = a.PullRequestEvent.Repo.GetName()
		orgRepo    = a.PullRequestEvent.Repo.GetFullName()
		linterName = a.LinterName
	)

	log.Infof("[%s] found total %d files with %d lint errors on repo %v", linterName, len(lintResults), countLinterErrors(lintResults), orgRepo)
	lintResults, err := Filters(log, a, lintResults)
	if err != nil {
		log.Errorf("failed to filter lint errors: %v", err)
		return err
	}

	log.Infof("[%s] found %d files with valid %d linter errors related to this PR %d (%s) \n", linterName, len(lintResults), countLinterErrors(lintResults), num, orgRepo)

	if len(lintResults) > 0 {
		metric.IncIssueCounter(orgRepo, linterName, a.PullRequestEvent.PullRequest.GetHTMLURL(), a.PullRequestEvent.GetPullRequest().GetHead().GetSHA(), float64(countLinterErrors(lintResults)))
	}

	switch a.LinterConfig.ReportFormat {
	case config.GithubCheckRuns:
		ch, err := CreateGithubChecks(context.Background(), a, lintResults)
		if err != nil {
			log.Errorf("failed to create github checks: %v", err)
			return err
		}
		log.Infof("[%s] create check run success, HTML_URL: %v", linterName, ch.GetHTMLURL())

		metric.NotifyWebhookByText(ConstructGotchaMsg(linterName, a.PullRequestEvent.GetPullRequest().GetHTMLURL(), ch.GetHTMLURL(), lintResults))
	case config.GithubPRReview:
		// List existing comments
		existedComments, err := ListPullRequestsComments(context.Background(), a.GithubClient, org, repo, num)
		if err != nil {
			log.Errorf("failed to list comments: %v", err)
			return err
		}

		// filter out the comments that are not related to the linter
		var existedCommentsToKeep []*github.PullRequestComment
		var linterFlag = linterNamePrefix(linterName)
		for _, comment := range existedComments {
			if strings.HasPrefix(comment.GetBody(), linterFlag) {
				existedCommentsToKeep = append(existedCommentsToKeep, comment)
			}
		}
		log.Infof("%s found %d existed comments for this PR %d (%s) \n", linterFlag, len(existedCommentsToKeep), num, orgRepo)

		toAdds, toDeletes := filterLinterOutputs(lintResults, existedCommentsToKeep)
		if err := DeletePullReviewComments(context.Background(), a.GithubClient, org, repo, toDeletes); err != nil {
			log.Errorf("failed to delete comments: %v", err)
			return err
		}
		log.Infof("%s delete %d comments for this PR %d (%s) \n", linterFlag, len(toDeletes), num, orgRepo)

		comments := constructPullRequestComments(toAdds, linterFlag, a.PullRequestEvent.GetPullRequest().GetHead().GetSHA())
		if len(comments) == 0 {
			return nil
		}

		// Add the comments
		addedCmts, err := CreatePullReviewComments(context.Background(), a.GithubClient, org, repo, num, comments)
		if err != nil {
			log.Errorf("failed to post comments: %v", err)
			return err
		}
		log.Infof("[%s] add %d comments for this PR %d (%s) \n", linterName, len(addedCmts), num, orgRepo)
		metric.NotifyWebhookByText(ConstructGotchaMsg(linterName, a.PullRequestEvent.GetPullRequest().GetHTMLURL(), addedCmts[0].GetHTMLURL(), lintResults))
	case config.Quiet:
		return nil
	default:
		log.Errorf("unsupported report format: %v", a.LinterConfig.ReportFormat)
	}

	return nil
}

// LineParser is a function that parses a line of linter output.
type LineParser func(line string) (*LinterOutput, error)

// Parse parses the output of a linter command.
func Parse(log *xlog.Logger, output []byte, lineParser LineParser) (results map[string][]LinterOutput, unexpected []string) {
	lines := strings.Split(string(output), "\n")
	results = make(map[string][]LinterOutput, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		output, err := lineParser(line)
		if err != nil {
			unexpected = append(unexpected, line)
			continue
		}

		if output == nil {
			continue
		}

		if outs, ok := results[output.File]; !ok {
			results[output.File] = []LinterOutput{*output}
		} else {
			// remove duplicate
			var existed bool
			for _, v := range outs {
				if v.File == output.File && v.Line == output.Line && v.Column == output.Column && v.Message == output.Message {
					existed = true
					break
				}
			}

			if !existed {
				results[output.File] = append(results[output.File], *output)
			}
		}
	}
	return
}

// trainer is a function that trains the linter output.
// It returns the trained linter output and unexpected lines.
type trainer func(LinterOutput) (LinterOutput, []string)

// ParseV2 parses the output of a linter command.
// It returns the structured lint results `file:line:column: message` if parsed successfully and unexpected lines if failed.
// Message can be multi-line, column is optional, the unexpected lines are the lines that cannot be parsed.
// Use trainer if there is any special case for the linter output. e.g. mark (typecheck) as unexpected.
func ParseV2(log *xlog.Logger, output []byte, trainer func(LinterOutput) (*LinterOutput, []string)) (map[string][]LinterOutput, []string) {
	pattern := `(.*?):(\d+):(\d+)?:?`
	regex := regexp.MustCompile(pattern)
	indices := regex.FindAllStringSubmatchIndex(string(output), -1)
	if len(indices) == 0 {
		return nil, strings.Split(string(output), "\n")
	}

	var unexpected = make([]string, 0, len(indices))
	var prefix string
	// get the prefix before the first issue which generally is some unexpected message.
	if len(indices) > 0 && indices[0][0] > 0 {
		prefix = strings.TrimSpace(string(output[:indices[0][0]]))
		if prefix != "" {
			unexpected = append(unexpected, strings.Split(prefix, "\n")...)
		}
	}

	var results = make(map[string][]LinterOutput, len(indices))
	for i := 0; i < len(indices); i++ {
		issue := LinterOutput{
			File: string(output[indices[i][2]:indices[i][3]]),
		}

		line, err := strconv.ParseInt(string(output[indices[i][4]:indices[i][5]]), 10, 64)
		if err != nil {
			log.Errorf("unexpected line number: %s, err: %v", string(output[indices[i][4]:indices[i][5]]), err)
			continue
		}
		issue.Line = int(line)

		var msgStart = indices[i][5] + 1

		// column is optional in some linters' output
		if indices[i][6] != -1 && indices[i][7] != -1 {
			column, err := strconv.ParseInt(string(output[indices[i][6]:indices[i][7]]), 10, 64)
			if err != nil {
				log.Errorf("unexpected column number: %s, err: %v", string(output[indices[i][6]:indices[i][7]]), err)
				continue
			}
			issue.Column = int(column)
			msgStart = indices[i][7] + 1
		}

		if msgStart < len(output) {
			if i+1 < len(indices) {
				issue.Message = strings.TrimSpace(string(output[msgStart:indices[i+1][0]]))
			} else {
				issue.Message = strings.TrimSpace(string(output[msgStart:]))
			}
		}

		if trainer != nil {
			t, u := trainer(issue)
			if len(u) > 0 {
				unexpected = append(unexpected, u...)
			}

			if t == nil {
				// skip this issue
				continue
			}
			issue = *t
		}

		if outputs, ok := results[issue.File]; !ok {
			results[issue.File] = []LinterOutput{issue}
		} else {
			// remove duplicate
			var existed bool
			for _, v := range outputs {
				if v.File == issue.File && v.Line == issue.Line && v.Column == issue.Column && v.Message == issue.Message {
					existed = true
					break
				}
			}
			if !existed {
				results[issue.File] = append(outputs, issue)
			}
		}
	}

	return results, unexpected
}

// common format LinterLine
func GeneralLineParser(line string) (*LinterOutput, error) {
	pattern := `^(.*?):(\d+):(\d+)?:? (.*)$`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex: %v, err: %v", pattern, err)
	}
	matches := regex.FindStringSubmatch(line)

	if len(matches) != 5 {
		return nil, fmt.Errorf("unexpected format, original: %s", line)
	}

	lineNumber, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unexpected line number: %s, err: %v, original line: %v", matches[2], err, line)
	}

	var columnNumber int64

	if matches[3] != "" {
		column, err := strconv.ParseInt(matches[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unexpected column number: %s, err: %v, original line: %v", matches[3], err, line)
		}
		columnNumber = column

	}
	return &LinterOutput{
		File:    matches[1],
		Line:    int(lineNumber),
		Column:  int(columnNumber),
		Message: matches[4],
	}, nil
}

func isGeneratedFile(file string) (bool, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return false, err
	}

	dataStr := string(data)
	return strings.Contains(dataStr, "Code generated by") || strings.Contains(dataStr, "DO NOT EDIT"), nil
}

func IsEmpty(args ...string) bool {
	for _, arg := range args {
		if arg != "" {
			return false
		}
	}
	return true
}

func countLinterErrors(lintResults map[string][]LinterOutput) int {
	var count int
	for _, outputs := range lintResults {
		count += len(outputs)
	}
	return count
}

func ConstructGotchaMsg(linter, pr, link string, linterResults map[string][]LinterOutput) string {
	if len(linterResults) == 0 {
		return ""
	}

	var message string
	for file, outputs := range linterResults {
		for _, output := range outputs {
			message += fmt.Sprintf("%s:%d:%d: %s\n", file, output.Line, output.Column, output.Message)
		}
	}

	return fmt.Sprintf("✅ Linter: %v \nPR:   %v \nLink: %v \nDetails:\n%v\n", linter, pr, link, message)
}

func ConstructUnknownMsg(linter, repo, pr, event, unexpected string) string {
	return fmt.Sprintf("😱🚀 Linter: %v \nRepo: %v \nPR:   %v \nEvent: %v \nUnexpected: %v\n", linter, repo, pr, event, unexpected)
}
