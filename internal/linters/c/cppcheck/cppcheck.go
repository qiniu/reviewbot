package cppcheck

import (
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

// refer to https://cppcheck.sourceforge.io/
var linterName = "cppcheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, cppcheckHandler)
	// see https://stackoverflow.com/a/3223792/5057547
	linters.RegisterLinterLanguages(linterName, []string{".c", ".cpp", ".h", ".hpp", ".cc", ".cxx", ".hxx", ".c++"})
}

func cppcheckHandler(log *xlog.Logger, a linters.Agent) error {
	if linters.IsEmpty(a.LinterConfig.Args...) {
		a.LinterConfig.Args = append([]string{}, "--quiet", "--template='{file}:{line}:{column}: {message}'", ".")
	}

	return linters.GeneralHandler(log, a, func(l *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
		return linters.Parse(log, output, cppcheckParser)
	})
}

func cppcheckParser(line string) (*linters.LinterOutput, error) {
	lineResult, err := linters.GeneralLineParser(line)
	if err != nil {
		return nil, err

	}
	return &linters.LinterOutput{
		File:    strings.TrimLeft(lineResult.File, "'"),
		Line:    lineResult.Line,
		Column:  lineResult.Column,
		Message: strings.TrimRight(lineResult.Message, "'"),
	}, nil
}
