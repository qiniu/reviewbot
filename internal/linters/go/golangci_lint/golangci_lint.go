package golangci_lint

import (
	"fmt"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

// refer to https://golangci-lint.run/
var lintName = "golangci-lint"

func init() {
	linters.RegisterPullRequestHandler(lintName, golangciLintHandler)
	linters.RegisterLinterLanguages(lintName, []string{"go"})
}

func golangciLintHandler(log *xlog.Logger, a linters.Agent) error {
	if linters.IsEmpty(a.LinterConfig.Args...) {
		a.LinterConfig.Args = append([]string{}, "run")
	}

	return linters.GeneralHandler(log, a, golangciLintParse)
}

func golangciLintParse(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	var lineParse = func(line string) (*linters.LinterOutput, error) {
		if strings.HasSuffix(line, "(typecheck)") {
			// refer: https://github.com/qiniu/reviewbot/issues/82#issuecomment-2002340788
			return nil, fmt.Errorf("skip golangci-lint typecheck error: %s", line)
		}

		return linters.GeneralLineParser(line)
	}

	return linters.Parse(log, output, lineParse)
}
