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
	"fmt"

	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/runner"
	"github.com/qiniu/reviewbot/internal/storage"
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
	// RepoDir is the repo directory.
	RepoDir string
	// GenLogKey generates the log key.
	GenLogKey func() string
	// GenLogViewURL generates the log view url.
	GenLogViewURL func() string
	// IssueReferences is the compiled issue references config for the linter.
	IssueReferences []config.CompiledIssueReference
}

// ApplyIssueReferences applies the issue references to the lint results.
func (a *Agent) ApplyIssueReferences(lintResults map[string][]LinterOutput) {
	msgFormat := "%s\nmore info: %s"
	for _, ref := range a.IssueReferences {
		for file, outputs := range lintResults {
			for i, o := range outputs {
				if ref.Pattern.MatchString(o.Message) {
					lintResults[file][i].Message = fmt.Sprintf(msgFormat, o.Message, ref.URL)
				}
			}
		}
	}
}
