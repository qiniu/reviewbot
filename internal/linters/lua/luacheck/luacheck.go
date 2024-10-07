/*
 Copyright 2024 Qiniu Cloud (qiniu.com).

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package luacheck

import (
	"context"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/xlog"
)

// refer to https://github.com/mpeterv/luacheck
const linterName = "luacheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, luacheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".lua"})
}

func luacheckHandler(ctx context.Context, a linters.Agent) error {
	log := lintersutil.FromContext(ctx)
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
	return linters.GeneralHandler(ctx, log, a, linters.ExecRun, parser)
}

func parser(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
	lineParse := func(line string) (*linters.LinterOutput, error) {
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
