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
	"time"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/cache"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/reviewbot/internal/llm"
	"github.com/qiniu/reviewbot/internal/runner"
	"github.com/qiniu/reviewbot/internal/storage"
	"github.com/qiniu/x/log"
	"github.com/tmc/langchaingo/llms"
)

// issueCache is the issue references cache.
var issueCache = cache.NewIssueReferencesCache(time.Hour * 2)

var (
	ErrIssueNumberIsZero         = errors.New("issue number is 0, this should not happen except for test cases")
	ErrFailedToFetchIssueContent = errors.New("failed to fetch issue content")
)

// Agent knows necessary information in order to run linters.
type Agent struct {
	// ID each linter execution has a unique id.
	ID string
	// Storage knows how to store and retrieve the linter logs.
	Storage storage.Storage
	// Runner is the way to run the linter.	like docker, local, etc.
	Runner runner.Runner
	// Provider knows how to interact with the git provider. such as github, gitlab, etc.
	Provider Provider
	// LinterConfig is the linter configuration.
	LinterConfig config.Linter
	// RepoDir is the main repo directory.
	RepoDir string
	// GenLogKey generates the log key.
	GenLogKey func() string
	// GenLogViewURL generates the log view url.
	GenLogViewURL func() string
	// IssueReferences is the compiled issue references config for the linter.
	IssueReferences []config.CompiledIssueReference
	// ModelClient is the LLM model client.
	ModelClient llms.Model
}

// getMsgFormat returns the message format based on report type.
func getMsgFormat(format config.ReportType) string {
	switch format {
	case config.GitHubPRReview, config.GitHubMixType:
		return "[%s](%s)"
	default:
		return "%s\nmore info: %s"
	}
}

// getIssueContent fetches issue content with cache support.
func (a *Agent) getIssueContent(ctx context.Context, ref config.CompiledIssueReference) (string, error) {
	log := lintersutil.FromContext(ctx)

	// Try cache first
	if content, ok := issueCache.Get(ref.URL); ok && !issueCache.IsExpired(ref.URL) {
		return content, nil
	}

	if ref.IssueNumber == 0 {
		return "", ErrIssueNumberIsZero
	}

	// Fetch from GitHub
	issue, resp, err := github.NewClient(http.DefaultClient).Issues.Get(ctx, "qiniu", "reviewbot", ref.IssueNumber)
	if err != nil {
		log.Errorf("failed to fetch issue content: %s", err)
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		log.Errorf("failed to fetch issue content, resp: %v", resp)
		return "", ErrFailedToFetchIssueContent
	}

	content := issue.GetBody()
	issueCache.Set(ref.URL, content)
	return content, nil
}

// processOutput processes a single lint output.
func (a *Agent) processOutput(ctx context.Context, output LinterOutput, ref config.CompiledIssueReference, msgFormat string) (LinterOutput, bool) {
	if !ref.Pattern.MatchString(output.Message) {
		return output, false
	}

	newOutput := output
	newOutput.TypedMessage = fmt.Sprintf(msgFormat, output.Message, ref.URL)

	// Add issue content for PR review formats
	if a.LinterConfig.ReportType == config.GitHubPRReview || a.LinterConfig.ReportType == config.GitHubMixType {
		if content, err := a.getIssueContent(ctx, ref); err == nil {
			newOutput.TypedMessage += fmt.Sprintf(ReferenceFooter, content)
		}
	}

	return newOutput, true
}

// ApplyTypedMessageByIssueReferences applies the issue references to the lint results with the typed message.
func (a *Agent) ApplyTypedMessageByIssueReferences(ctx context.Context, lintResults map[string][]LinterOutput) map[string][]LinterOutput {
	msgFormat := getMsgFormat(a.LinterConfig.ReportType)
	newLintResults := make(map[string][]LinterOutput, len(lintResults))

	// Process each file's outputs
	for file, outputs := range lintResults {
		newOutputs := make([]LinterOutput, 0, len(outputs))

		// Apply each reference pattern
		for _, output := range outputs {
			processed := false
			for _, ref := range a.IssueReferences {
				if newOutput, ok := a.processOutput(ctx, output, ref, msgFormat); ok {
					newOutputs = append(newOutputs, newOutput)
					processed = true
					break
				}
			}
			if !processed {
				resp, err := llm.QueryForReference(ctx, a.ModelClient, output.Message)
				if err != nil {
					log.Errorf("failed to query LLM server: %v", err)
				}
				output.Message += fmt.Sprintf(ReferenceFooter, resp)
				newOutputs = append(newOutputs, output)
			}
		}

		if len(newOutputs) > 0 {
			newLintResults[file] = newOutputs
		}
	}

	return newLintResults
}

const ReferenceFooter = `
<details>
<summary>Details</summary>

%s
</details>`
