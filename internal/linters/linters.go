/*
Copyright 2023 Qiniu Cloud (qiniu.com).

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
	"github.com/cr-bot/config"
	"github.com/google/go-github/v57/github"
	gitv2 "k8s.io/test-infra/prow/git/v2"
)

var (
	codeReviewHandlers = map[string]CodeReviewHandlerFunc{}
	commentHandlers    = map[string]CommentHandlerFunc{}
)

// CommentHandlerFunc knows how to comment on a PR.
type CommentHandlerFunc func(config.Linter, Agent, github.PullRequestEvent) error

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

// CodeReviewHandlerFunc knows how to code review on a PR.
type CodeReviewHandlerFunc func(config.Linter, Agent, github.PullRequestEvent) (map[string][]LinterOutput, error)

// RegisterCodeReviewHandler registers a CodeReviewHandlerFunc for the given linter name.
func RegisterCodeReviewHandler(name string, handler CodeReviewHandlerFunc) {
	codeReviewHandlers[name] = handler
}

// LintHandlers returns a LinterHandlerFunc for the given linter name.
func LintHandlers(name string) []interface{} {
	var handlers []interface{}
	if handler, ok := codeReviewHandlers[name]; ok {
		handlers = append(handlers, handler)
	}

	if handler, ok := commentHandlers[name]; ok {
		handlers = append(handlers, handler)
	}

	return handlers
}

// Linter knows how to execute linters.
type Linter interface {
	// Run executes a linter command.
	Run(args ...string) ([]byte, error)
	// Parse parses the output of a linter command.
	Parse(output []byte) (map[string][]LinterOutput, error)
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
}

// Agent knows necessary information from cr-bot.
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
