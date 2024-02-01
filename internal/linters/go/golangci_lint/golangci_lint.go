package golangci_lint

import (
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

// refer to https://golangci-lint.run/
var lintName = "golangci-lint"

func init() {
	linters.RegisterPullRequestHandler(lintName, golangciLintHandler)
}

func golangciLintHandler(log *xlog.Logger, a linters.Agent) error {
	if linters.IsEmpty(a.LinterConfig.Args...) {
		// turn off compile errors by default
		a.LinterConfig.Args = append([]string{}, "run")
	}

	return linters.GeneralHandler(log, a, linters.GeneralParse)
}
