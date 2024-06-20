package luacheck

import (
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

// refer to https://github.com/mpeterv/luacheck
const linterName = "luacheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, luacheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".lua"})
}

func luacheckHandler(log *xlog.Logger, a linters.Agent) error {
	if linters.IsEmpty(a.LinterConfig.Args...) {
		// identify global variables for Redis and Nginx modules.
		// disable the maximum line length check, which is no need.
		// luacheck execute the command "--globals='ngx KEYS ARGV table redis cjson'", which does not take effect. So the parameter should be changed to an array-based approach.
		// Luacheck will output lines starting with unrecognized characters, so add the parameter: --no-color
		cmdArgs := []string{
			".",
			"--globals=ngx",
			"--globals=KEYS",
			"--globals=ARGV",
			"--globals=table",
			"--globals=redis",
			"--globals=cjson",
			"--no-max-line-length",
			"--no-color",
		}
		a.LinterConfig.Args = append([]string{}, cmdArgs...)
	}

	return linters.GeneralHandler(log, a, func(_ *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
		return linters.Parse(log, output, luacheckLineParser)
	})
}

func luacheckLineParser(line string) (*linters.LinterOutput, error) {
	filteredLine := luacheckLineProcess(line)
	if filteredLine != "" {
		return linters.GeneralLineParser(filteredLine)
	}
	return nil, nil
}

func luacheckLineProcess(line string) string {
	// Luacheck will output lines starting with 'Total ' or 'Checking ', which are no meaningful for the reviewbot scenario, so we discard them
	// Such as:
	// 1. Total: 0 warnings / 0 errors in 0 files
	// 2. Checking cmd/jarviswsserver/etc/get_node_wsserver.lua 11 warnings
	if strings.HasPrefix(line, "Total: ") || strings.HasPrefix(line, "Checking ") {
		return ""
	}
	return strings.TrimLeft(line, " ")
}
