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
		// luacheck execute the command "--globals='ngx KEYS ARGV table redis cjson'", which does not take effect.
		// so the parameter should be changed to an array-based approach.
		// luacheck will output lines starting with unrecognized characters, so add the parameter: --no-color
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

	// recommend to use the line-number format and disable the issued lines, since these are more friendly to the reviewbot
	// checking on luacheck 0.26.1 Lua5.1, there is no problem even with multiple --no-color parameter,
	// so we can add the parameter directly
	a.LinterConfig.Args = append(a.LinterConfig.Args, "--no-color")
	return linters.GeneralHandler(log, a, parser)
}

func parser(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
	var lineParse = func(line string) (*linters.LinterOutput, error) {
		// luacheck will output lines starting with 'Total ' or 'Checking '
		// which are no meaningful for the reviewbot scenario, so we discard them
		// such as:
		// 1. Total: 0 warnings / 0 errors in 0 files
		// 2. Checking cmd/jarviswsserver/etc/get_node_wsserver.lua 11 warnings
		// 3. Empty line
		if strings.HasPrefix(line, "Total: ") || strings.HasPrefix(line, "Checking ") || line == "" {
			return nil, nil
		}
		return linters.GeneralLineParser(strings.TrimLeft(line, " "))
	}
	return linters.Parse(log, output, lineParse)
}
