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
