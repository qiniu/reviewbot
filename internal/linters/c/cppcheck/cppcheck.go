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

	return linters.GeneralHandler(log, a, parser)
}

func parser(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	var lineParser = func(line string) (*linters.LinterOutput, error) {

		// skip the error output
		// If there are no detectable files in the current directory,
		// cppcheck will display the error message "cppcheck: error: could not find or open any of the paths given."
		// the error message is no meaningful for the reviewbot scenario, so we need to skip it

		if strings.Contains(line, "could not find or open any of the paths given") {
			return nil, nil
		}

		// cppcheck output characters ' at the beginning or end of its output
		if len(line) < 2 {
			return nil, nil
		}
		return linters.GeneralLineParser(line[1 : len(line)-1])
	}

	return linters.Parse(log, output, lineParser)
}
