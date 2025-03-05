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

package lint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/qiniu/reviewbot/internal/metric"
	"github.com/qiniu/reviewbot/internal/util"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

var (
	pullRequestHandlers = map[string]PullRequestHandlerFunc{}
	linterLanguages     = map[string][]string{}
)

// PullRequestHandlerFunc knows how to handle a pull request event.
type PullRequestHandlerFunc func(context.Context, Agent) error

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
	handlers := make(map[string]PullRequestHandlerFunc, len(pullRequestHandlers))
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
	// Message is the linter message
	Message string
	// StartLine required when using multi-line comments
	StartLine int
	// TypedMessage is the typed message
	TypedMessage string
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

func GeneralHandler(ctx context.Context, log *xlog.Logger, a Agent, execRun func(ctx context.Context, a Agent) ([]byte, error), linterParser func(*xlog.Logger, []byte) (map[string][]LinterOutput, []string)) error {
	linterName := a.LinterConfig.Name
	output, err := execRun(ctx, a)
	if err != nil {
		// NOTE(CarlJi): the error is *ExitError, it seems to have little information and needs to be handled in a better way.
		log.Warnf("%s run with exit code: %v, mark and continue", linterName, err)
	}

	// even if the output is empty, we still need to parse it
	// since we need delete the existed comments related to the linter

	lintResults, unexpected := linterParser(log, output)
	if len(unexpected) > 0 {
		msg := util.LimitJoin(unexpected, 1000)
		if msg != "" {
			// just log the unexpected lines and notify the webhook, no need to return error
			log.Warnf("unexpected lines: %v", msg)
			if !a.CLIMode {
				metric.NotifyWebhookByText(ConstructUnknownMsg(linterName, a.Provider.GetCodeReviewInfo().Org+"/"+a.Provider.GetCodeReviewInfo().Repo, a.Provider.GetCodeReviewInfo().URL, log.ReqId, msg))
			}
		}
	}

	return Report(ctx, a, lintResults)
}

// ExecRun executes a command.
func ExecRun(ctx context.Context, a Agent) ([]byte, error) {
	eventGuid := util.FromContext(ctx).ReqId
	start := time.Now()
	reader, err := a.Runner.Run(ctx, &a.LinterConfig)
	if err != nil {
		log.Warnf("failed to run linter: %v, mark and continue", err)
	}
	if reader == nil {
		return nil, fmt.Errorf("runner returned nil reader with error: %w", err)
	}
	defer reader.Close()

	output, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read linter output: %w", err)
	}

	end := time.Now()
	toLog := []byte(fmt.Sprintf("[%s][%s] run script:\n%s\n",
		start.Format(time.RFC3339), eventGuid, a.Runner.GetFinalScript()))
	toLog = append(toLog, []byte(fmt.Sprintf("[%s][%s] output:\n%s\n",
		end.Format(time.RFC3339), eventGuid, string(output)))...)
	err = a.Storage.Write(ctx, a.GenLogKey(), toLog)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Errorf("write to storage was failed %v", err)
		}
	}

	return output, nil
}

// GeneralParse parses the output of a linter command.
func GeneralParse(log *xlog.Logger, output []byte) (map[string][]LinterOutput, []string) {
	return Parse(log, output, GeneralLineParser)
}

// Report reports the lint results.
// This function should be always called even in custom linter handler since it will filter out the lint errors that are not related to the PR.
// and handle some special cases like auto-generated files.
func Report(ctx context.Context, a Agent, lintResults map[string][]LinterOutput) error {
	log := util.FromContext(ctx)

	if a.CLIMode {
		return a.CLIOutput(ctx, a.LinterConfig.Name, lintResults)
	}

	var (
		num        = a.Provider.GetCodeReviewInfo().Number
		orgRepo    = a.Provider.GetCodeReviewInfo().Org + "/" + a.Provider.GetCodeReviewInfo().Repo
		linterName = a.LinterConfig.Name
	)
	log.Infof("[%s] found total %d files with %d lint errors on repo %v", linterName, len(lintResults), countLinterErrors(lintResults), orgRepo)
	lintResults, err := Filters(log, a, lintResults)
	if err != nil {
		log.Errorf("failed to filter lint errors: %v", err)
		return err
	}

	log.Infof("[%s] found %d files with valid %d linter errors related to this PR %d (%s) \n", linterName, len(lintResults), countLinterErrors(lintResults), num, orgRepo)

	lintResults = a.EnrichWithIssueReferences(ctx, lintResults)
	if len(lintResults) > 0 {
		metric.IncIssueCounter(orgRepo, linterName, a.Provider.GetCodeReviewInfo().URL, a.Provider.GetCodeReviewInfo().HeadSHA, float64(countLinterErrors(lintResults)))
	}

	return a.Provider.Report(ctx, a, lintResults)
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

	unexpected := make([]string, 0, len(indices))
	var prefix string
	// get the prefix before the first issue which generally is some unexpected message.
	if len(indices) > 0 && indices[0][0] > 0 {
		prefix = strings.TrimSpace(string(output[:indices[0][0]]))
		if prefix != "" {
			unexpected = append(unexpected, strings.Split(prefix, "\n")...)
		}
	}

	results := make(map[string][]LinterOutput, len(indices))
	for i := 0; i < len(indices); i++ {
		file := strings.TrimSpace(string(output[indices[i][2]:indices[i][3]]))
		if strings.ContainsAny(file, " :") {
			log.Errorf("unexpected file name: %s", file)
			// skip the unexpected line
			continue
		}

		issue := LinterOutput{
			File: file,
		}

		line, err := strconv.ParseInt(string(output[indices[i][4]:indices[i][5]]), 10, 64)
		if err != nil {
			log.Errorf("unexpected line number: %s, err: %v", string(output[indices[i][4]:indices[i][5]]), err)
			continue
		}
		issue.Line = int(line)

		msgStart := indices[i][5] + 1

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

func GeneralLinterHandler(ctx context.Context, a Agent) error {
	log := util.FromContext(ctx)
	return GeneralHandler(ctx, log, a, ExecRun, GeneralParse)
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
