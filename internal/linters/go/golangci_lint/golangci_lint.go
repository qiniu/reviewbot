package golangcilint

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
	linters.RegisterLinterLanguages(lintName, []string{".go"})
}

func golangciLintHandler(log *xlog.Logger, a linters.Agent) error {
	if linters.IsEmpty(a.LinterConfig.Args...) {
		// refer: https://github.com/qiniu/reviewbot/issues/146
		a.LinterConfig.Args = append([]string{}, "run", "--timeout=5m0s", "--allow-parallel-runners=true")
	}

	// recommend to use the line-number format and disable the issued lines, since these are more friendly to the reviewbot
	// checking on golangci-lint 1.59.0, there is no problem even with multiple --out-format and --print-issued-lines parameters,
	// so we can add these parameters directly
	a.LinterConfig.Args = append(a.LinterConfig.Args, "--out-format=line-number", "--print-issued-lines=false")

	if a.LinterConfig.ConfigPath != "" {
		a.LinterConfig.Args = append(a.LinterConfig.Args, "--config", a.LinterConfig.ConfigPath)
	}

	return linters.GeneralHandler(log, a, parser)
}

func parser(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	var lineParser = func(line string) (*linters.LinterOutput, error) {
		if strings.HasSuffix(line, "(typecheck)") {
			// refer: https://github.com/qiniu/reviewbot/issues/82#issuecomment-2002340788
			return nil, fmt.Errorf("skip golangci-lint typecheck error: %s", line)
		}

		// skip the warning level log
		// example: level=warning msg="[linters_context] copyloopvar: this linter is disabled because the Go version (1.18) of your project is lower than Go 1.22"
		// the warning level log is not a real lint error, so we need to skip it
		if strings.Contains(line, "level=warning") {
			log.Warnf("skip golangci-lint warning: %s", line)
			return nil, nil
		}

		return linters.GeneralLineParser(line)
	}

	return linters.Parse(log, output, lineParser)
}
