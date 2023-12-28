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
)

var (
	lintersHandlers = map[string]LinterHandlerFunc{}
)

// LinterHandlerFunc knows how to run a linter.
type LinterHandlerFunc func(config.Linter) (map[string][]LinterOutput, error)

func RegisterLinter(name string, handler LinterHandlerFunc) {
	lintersHandlers[name] = handler
}

// LinterHandler returns a LinterHandlerFunc for the given linter name.
func LinterHandler(name string) LinterHandlerFunc {
	if handler, ok := lintersHandlers[name]; ok {
		return handler
	}
	return nil
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

	// Label is the staticcheck Label
	Label string
}
