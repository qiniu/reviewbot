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
	//StratLine required when using multi-line comments
	StratLine int
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

	if len(output) == 0 {
		return nil
	}

	lintResults, err := parse(log, output)
	if err != nil {
		log.Errorf("failed to parse output failed: %v, cmd: %v", err, cmd)
		return err
	}

	if len(lintResults) == 0 {
		return nil
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
	comments, err := BuildPullRequestCommentBody(linterName, lintResults, a.PullRequestChangedFiles)
	if err != nil {
		log.Errorf("failed to build pull request comment body: %v", err)
		return err
	}

	if len(comments) == 0 {
		// no related comments to post, continue to run other linters
		return nil
	}

	log.Infof("[%s] found valid %d comments related to this PR %d (%s) \n", linterName, len(comments), num, orgRepo)
	if err := PostPullReviewCommentsWithRetry(context.Background(), a.GithubClient, org, repo, num, comments); err != nil {
		log.Errorf("failed to post comments: %v", err)
		return err
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

		if isGopAutoGeneratedFile(output.File) {
			log.Infof("skip auto generated file by go+ : %s", output.File)
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

var gop_auto_generated_file_pattern = `^.*_autogen.*.go$`
var gopGeneratedFileRegexp = regexp.MustCompile(gop_auto_generated_file_pattern)

func isGopAutoGeneratedFile(file string) bool {
	// TODO: need a more accurate way to determine whether it is a go+ auto generated file
	return gopGeneratedFileRegexp.MatchString(filepath.Base(file))
}

func IsEmpty(args ...string) bool {
	for _, arg := range args {
		if arg != "" {
			return false
		}
	}

	return true
}
