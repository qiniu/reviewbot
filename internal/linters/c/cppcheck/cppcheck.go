package cppcheck

import (
	"context"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/xlog"
)

// refer to https://cppcheck.sourceforge.io/
var linterName = "cppcheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, cppcheckHandler)
	// see https://stackoverflow.com/a/3223792/5057547
	linters.RegisterLinterLanguages(linterName, []string{".c", ".cpp", ".h", ".hpp", ".cc", ".cxx", ".hxx", ".c++"})
}

func cppcheckHandler(ctx context.Context, a linters.Agent) error {
	log := lintersutil.FromContext(ctx)
	if linters.IsEmpty(a.LinterConfig.Args...) {
		a.LinterConfig.Args = append([]string{}, "--quiet", "--template='{file}:{line}:{column}: {message}'", ".")
	}

	return linters.GeneralHandler(ctx, log, a, linters.ExecRun, parser)
}

func parser(log *xlog.Logger, input []byte) (map[string][]linters.LinterOutput, []string) {
	lineParser := func(line string) (*linters.LinterOutput, error) {
		if len(line) <= 2 {
			return nil, nil
		}

		// remove the first and last character of the line,
		// which are the single quotes
		line = line[1 : len(line)-1]
		return linters.GeneralLineParser(line)
	}
	return linters.Parse(log, input, lineParser)
}
