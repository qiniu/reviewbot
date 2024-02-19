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
	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/x/xlog"
	gitv2 "k8s.io/test-infra/prow/git/v2"
)

var (
	codeReviewHandlers = map[string]CodeReviewHandlerFunc{}
	commentHandlers    = map[string]CommentHandlerFunc{}
)

// CommentHandlerFunc knows how to comment on a PR.
type CommentHandlerFunc func(*xlog.Logger, config.Linter, Agent, github.PullRequestEvent) error

// RegisterCommentHandler registers a CommentHandlerFunc for the given linter name.
func RegisterCommentHandler(name string, handler CommentHandlerFunc) {
	commentHandlers[name] = handler
}

// CommentHandler returns a CommentHandlerFunc for the given linter name.
func CommentHandler(name string) CommentHandlerFunc {
	if handler, ok := commentHandlers[name]; ok {
		return handler
	}
	return nil
}

// TotalCommentHandlers returns all registered CommentHandlerFunc.
func TotalCommentHandlers() map[string]CommentHandlerFunc {
	var handlers = make(map[string]CommentHandlerFunc, len(commentHandlers))
	for name, handler := range commentHandlers {
		handlers[name] = handler
	}

	return handlers
}

// CodeReviewHandlerFunc knows how to code review on a PR.
type CodeReviewHandlerFunc func(*xlog.Logger, config.Linter, Agent, github.PullRequestEvent) (map[string][]LinterOutput, error)

// RegisterCodeReviewHandler registers a CodeReviewHandlerFunc for the given linter name.
func RegisterCodeReviewHandler(name string, handler CodeReviewHandlerFunc) {
	codeReviewHandlers[name] = handler
}

// CodeReviewHandler returns a CodeReviewHandlerFunc for the given linter name.
func TotalCodeReviewHandlers() map[string]CodeReviewHandlerFunc {
	var handlers = make(map[string]CodeReviewHandlerFunc, len(codeReviewHandlers))
	for name, handler := range codeReviewHandlers {
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

// Agent knows necessary information from reviewbot.
type Agent struct {
	gc               *github.Client
	gitClientFactory gitv2.ClientFactory
	config           config.Config
}

func NewAgent(gc *github.Client, gitClientFactory gitv2.ClientFactory, config config.Config) Agent {
	return Agent{
		gc:               gc,
		gitClientFactory: gitClientFactory,
		config:           config,
	}
}

func (a Agent) GitHubClient() *github.Client {
	return a.gc
}

func (a Agent) GitClientFactory() gitv2.ClientFactory {
	return a.gitClientFactory
}

func (a Agent) Config() config.Config {
	return a.config
}

const CommentFooter = `
<details>

If you have any questions about this comment, feel free to raise an issue here:

- **https://github.com/qiniu/reviewbot**

</details>`
