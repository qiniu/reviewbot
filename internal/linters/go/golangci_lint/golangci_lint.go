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
	a = args(a)
	return linters.GeneralHandler(log, a, parser)
}

func parser(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
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

// args is used to set the default parameters for golangci-lint
// see: ./docs/website/docs/component/go/golangci-lint
func args(a linters.Agent) linters.Agent {
	config := a.LinterConfig
	if len(config.Command) == 0 || len(config.Command) > 1 || config.Command[0] != lintName {
		return a
	}

	legacyArgs := config.Args

	switch {
	case len(legacyArgs) == 0:
		legacyArgs = []string{}
	case len(legacyArgs) > 0 && legacyArgs[0] != "run":
		return a
	default:
		legacyArgs = legacyArgs[1:]
	}

	var newArgs = []string{"run"}

	var (
		timeoutFlag   bool
		parallelFlag  bool
		outFormatFlag bool
		printFlag     bool
		configFlag    bool
	)

	for _, arg := range legacyArgs {

		switch {
		case strings.HasPrefix(arg, "--timeout"):
			timeoutFlag = true
		case strings.HasPrefix(arg, "--allow-parallel-runners"):
			parallelFlag = true
		case strings.HasPrefix(arg, "--out-format"):
			outFormatFlag = true
		case strings.HasPrefix(arg, "--print-issued-lines"):
			printFlag = true
		case strings.HasPrefix(arg, "--config"):
			configFlag = true
		}

		newArgs = append(newArgs, arg)
	}

	if !timeoutFlag {
		newArgs = append(newArgs, "--timeout=5m0s")
	}
	if !parallelFlag {
		newArgs = append(newArgs, "--allow-parallel-runners=true")
	}
	if !outFormatFlag {
		newArgs = append(newArgs, "--out-format=line-number")
	}
	if !printFlag {
		newArgs = append(newArgs, "--print-issued-lines=false")
	}
	if !configFlag && config.ConfigPath != "" {
		newArgs = append(newArgs, "--config", config.ConfigPath)
	}

	config.Args = newArgs
	a.LinterConfig = config

	return a
}
