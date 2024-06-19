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
		a.LinterConfig.Args = append([]string{}, ".", "--globals='ngx KEYS ARGV table redis cjson'", "--no-max-line-length")
	}

	return linters.GeneralHandler(log, a, func(_ *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
		return linters.Parse(log, output, luacheckParser)
	})
}

func luacheckParser(line string) (*linters.LinterOutput, error) {
	lineResult, err := linters.GeneralLineParser(line)
	if err != nil {
		return nil, err

	}
	return &linters.LinterOutput{
		File:    strings.TrimLeft(lineResult.File, " "),
		Line:    lineResult.Line,
		Column:  lineResult.Column,
		Message: strings.ReplaceAll(strings.ReplaceAll(lineResult.Message, "\x1b[0m\x1b[1m", ""), "\x1b[0m", ""),
	}, nil
}
