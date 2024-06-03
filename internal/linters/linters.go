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
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
	gitv2 "k8s.io/test-infra/prow/git/v2"
)

//go:embed resouces/*
var resources embed.FS
var (
	PullRequestHandlers = map[string]PullRequestHandlerFunc{}
	LinterLanguages     = map[string][]string{}
)

// PullRequestHandlerFunc knows how to handle a pull request event.
type PullRequestHandlerFunc func(*xlog.Logger, Agent) error

// RegisterPullRequestHandler registers a PullRequestHandlerFunc for the given linter name.
func RegisterPullRequestHandler(name string, handler PullRequestHandlerFunc) {
	PullRequestHandlers[name] = handler
}

// RegisterLinterLanguages registers the languages supported by the linter.
func RegisterLinterLanguages(name string, languages []string) {
	LinterLanguages[name] = languages
}

// PullRequestHandler returns a PullRequestHandlerFunc for the given linter name.
func PullRequestHandler(name string) PullRequestHandlerFunc {
	if handler, ok := PullRequestHandlers[name]; ok {
		return handler
	}
	return nil
}

// TotalPullRequestHandlers returns all registered PullRequestHandlerFunc.
func TotalPullRequestHandlers() map[string]PullRequestHandlerFunc {
	var handlers = make(map[string]PullRequestHandlerFunc, len(PullRequestHandlers))
	for name, handler := range PullRequestHandlers {
		handlers[name] = handler
	}

	return handlers
}

// LinterLanguages returns the languages supported by the linter.
func Languages(linterName string) []string {
	return LinterLanguages[linterName]
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
}

const CommentFooter = `
<details>

If you have any questions about this comment, feel free to raise an issue here:

- **https://github.com/qiniu/reviewbot**

</details>`

// ------------------------ Linter ------------------------

func GeneralHandler(log *xlog.Logger, a Agent, ParseFunc func(*xlog.Logger, []byte) (map[string][]LinterOutput, error)) error {
	cmd := a.LinterConfig.Command
	output, err := ExecRun(a.LinterConfig.WorkDir, cmd, a.LinterConfig.Args...)
	if err != nil {
		log.Errorf("%s run failed: %v, mark and continue", cmd, err)
	}

	// even if the output is empty, we still need to parse it
	// since we need delete the existed comments related to the linter

	lintResults, err := ParseFunc(log, output)
	if err != nil {
		log.Errorf("failed to parse output failed: %v, cmd: %v", err, cmd)
		return err
	}

	return Report(log, a, lintResults)
}

// ExecRun executes a command.
func ExecRun(workDir, command string, args ...string) ([]byte, error) {
	g, err := exec.LookPath(command)
	if err != nil {
		return nil, err
	}
	c := exec.Command(g, args...)
	c.Dir = workDir
	return c.CombinedOutput()
}

// GeneralParse parses the output of a linter command.
func GeneralParse(log *xlog.Logger, output []byte) (map[string][]LinterOutput, error) {
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
		linterName = a.LinterConfig.LinterName
	)

	log.Infof("[%s] found total %d files with %d lint errors on repo %v", linterName, len(lintResults), countLinterErrors(lintResults), orgRepo)
	lintResults, err := Filters(log, a, lintResults)
	if err != nil {
		log.Errorf("failed to filter lint errors: %v", err)
		return err
	}

	log.Infof("[%s] found %d files with valid %d linter errors related to this PR %d (%s) \n", linterName, len(lintResults), countLinterErrors(lintResults), num, orgRepo)

	// only not report when there is no lint errors and the linter is not related to the PR
	// see https://github.com/qiniu/reviewbot/issues/108#issuecomment-2042217108
	if len(lintResults) == 0 && !languageRelated(linterName, exts(a.PullRequestChangedFiles)) {
		log.Debugf("[%s] no lint errors found and not languages related, skip report", linterName)
		return nil
	}

	switch a.LinterConfig.ReportFormat {
	case config.GithubCheckRuns:
		ch, err := CreateGithubChecks(context.Background(), a, lintResults)
		if err != nil {
			log.Errorf("failed to create github checks: %v", err)
			return err
		}
		log.Infof("[%s] create check run success, HTML_URL: %v", linterName, ch.GetHTMLURL())
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
		// Add the comments
		if err := CreatePullReviewComments(context.Background(), a.GithubClient, org, repo, num, comments); err != nil {
			log.Errorf("failed to post comments: %v", err)
			return err
		}
		log.Infof("[%s] add %d comments for this PR %d (%s) \n", linterName, len(toAdds), num, orgRepo)

	default:
		log.Errorf("unsupported report format: %v", a.LinterConfig.ReportFormat)
	}

	return nil
}

// LineParser is a function that parses a line of linter output.
type LineParser func(line string) (*LinterOutput, error)

// Parse parses the output of a linter command.
func Parse(log *xlog.Logger, output []byte, lineParser LineParser) (map[string][]LinterOutput, error) {
	lines := strings.Split(string(output), "\n")

	var result = make(map[string][]LinterOutput)
	for _, line := range lines {
		if line == "" {
			continue
		}
		output, err := lineParser(line)
		if err != nil {
			log.Warnf("unexpected linter check output: %v", line)
			continue
		}

		if output == nil {
			continue
		}

		if outs, ok := result[output.File]; !ok {
			result[output.File] = []LinterOutput{*output}
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
				result[output.File] = append(result[output.File], *output)
			}
		}
	}

	return result, nil
}

// common format LinterLine

func GeneralLineParser(line string) (*LinterOutput, error) {
	log.Debugf("parse line: %s", line)
	patternColum := `^(.*):(\d+):(\d+): (.*)$`
	regexColum, errColum := regexp.Compile(patternColum)
	if errColum != nil {
		log.Errorf("compile regex failed: %v", errColum)
		return nil, errColum
	}
	pattern := `^(.*):(\d+): (.*)$`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		log.Errorf("compile regex failed: %v", err)
		return nil, err
	}
	matches := regexColum.FindStringSubmatch(line)
	if matches == nil {
		matches = regex.FindStringSubmatch(line)
	}
	if len(matches) < 4 {
		return nil, fmt.Errorf("unexpected format, original: %s", line)
	}

	lineNumber, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	var column int64
	message := matches[3]
	if len(matches) > 4 {
		columnNumber, err := strconv.ParseInt(matches[3], 10, 64)
		if err == nil {
			column = columnNumber
		}
		message = matches[4]

	}
	return &LinterOutput{
		File:    matches[1],
		Line:    int(lineNumber),
		Column:  int(column),
		Message: message,
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

func exts(changes []*github.CommitFile) map[string]bool {
	var exts = make(map[string]bool)
	for _, change := range changes {
		ext := filepath.Ext(change.GetFilename())
		if ext == "" {
			continue
		}
		exts[ext] = true
	}
	return exts
}

func languageRelated(linterName string, exts map[string]bool) bool {
	for _, language := range Languages(linterName) {
		if exts[language] {
			return true
		}
	}
	return false
}

func countLinterErrors(lintResults map[string][]LinterOutput) int {
	var count int
	for _, outputs := range lintResults {
		count += len(outputs)
	}
	return count
}
