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

var (
	PullRequestHandlers = map[string]PullRequestHandlerFunc{}
)

// PullRequestHandlerFunc knows how to handle a pull request event.
type PullRequestHandlerFunc func(*xlog.Logger, Agent) error

// RegisterPullRequestHandler registers a PullRequestHandlerFunc for the given linter name.
func RegisterPullRequestHandler(name string, handler PullRequestHandlerFunc) {
	PullRequestHandlers[name] = handler
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

func GeneralHandler(log *xlog.Logger, a Agent, parse func(*xlog.Logger, []byte) (map[string][]LinterOutput, error)) error {
	cmd := a.LinterConfig.Command
	output, err := ExecRun(a.LinterConfig.WorkDir, cmd, a.LinterConfig.Args...)
	if err != nil {
		log.Errorf("%s run failed: %v, mark and continue", cmd, err)
	}

	// even if the output is empty, we still need to parse it
	// since we need delete the existed comments related to the linter

	lintResults, err := parse(log, output)
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

	return c.Output()
}

// GeneralParse parses the output of a linter command.
func GeneralParse(log *xlog.Logger, output []byte) (map[string][]LinterOutput, error) {
	return Parse(log, output, GeneralLineParser)
}

// Report reports the lint results.
func Report(log *xlog.Logger, a Agent, lintResults map[string][]LinterOutput) error {
	var (
		org        = a.PullRequestEvent.Repo.GetOwner().GetLogin()
		repo       = a.PullRequestEvent.Repo.GetName()
		num        = a.PullRequestEvent.GetNumber()
		orgRepo    = a.PullRequestEvent.Repo.GetFullName()
		linterName = a.LinterConfig.Command
	)

	log.Infof("[%s] found total %d files with lint errors on repo %v", linterName, len(lintResults), orgRepo)
	changedCodeLinterResults, err := filterLintErrs(lintResults, a.PullRequestChangedFiles)
	if err != nil {
		log.Errorf("failed to filter lint errors: %v", err)
		return err
	}

	// ignore generated files
	var filesToIgnore []string
	for file := range changedCodeLinterResults {
		absPath := filepath.Join(a.LinterConfig.WorkDir, file)
		if isGenerated, err := isGeneratedFile(absPath); err != nil {
			log.Errorf("failed to check if file is generated: %v", err)
			continue
		} else {
			if isGenerated {
				log.Infof("ignore generated file: %s", file)
				filesToIgnore = append(filesToIgnore, file)
			}
		}
	}

	for _, file := range filesToIgnore {
		delete(changedCodeLinterResults, file)
	}

	log.Infof("[%s] found valid %d linter errors related to this PR %d (%s) \n", linterName, len(changedCodeLinterResults), num, orgRepo)

	switch a.LinterConfig.ReportFormat {
	case config.GithubCheckRuns:
		if err := CreateGithubChecks(context.Background(), a, changedCodeLinterResults); err != nil {
			log.Errorf("failed to create github checks: %v", err)
			return err
		}
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

		toAdds, toDeletes := filterLinterOutputs(changedCodeLinterResults, existedCommentsToKeep)
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
	pattern := `^(.*):(\d+):(\d+): (.*)$`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		log.Errorf("compile regex failed: %v", err)
		return nil, err
	}
	matches := regex.FindStringSubmatch(line)
	if len(matches) != 5 {
		return nil, fmt.Errorf("unexpected format, original: %s", line)
	}

	lineNumber, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return nil, err
	}

	columnNumber, err := strconv.ParseInt(matches[3], 10, 64)
	if err != nil {
		return nil, err
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
