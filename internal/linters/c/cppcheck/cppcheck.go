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

package cppcheck

import (
	"context"

	"github.com/qiniu/reviewbot/internal/lint"
	"github.com/qiniu/reviewbot/internal/util"
	"github.com/qiniu/x/xlog"
)

// refer to https://cppcheck.sourceforge.io/
var linterName = "cppcheck"

func init() {
	lint.RegisterPullRequestHandler(linterName, cppcheckHandler)
	// see https://stackoverflow.com/a/3223792/5057547
	lint.RegisterLinterLanguages(linterName, []string{".c", ".cpp", ".h", ".hpp", ".cc", ".cxx", ".hxx", ".c++"})
}

func cppcheckHandler(ctx context.Context, a lint.Agent) error {
	log := util.FromContext(ctx)
	if lint.IsEmpty(a.LinterConfig.Args...) {
		// The check-level parameter has been supported since version 2.11, with the default normal mode.
		// In exhaustive mode, Cppcheck performs additional inspection rules and more complex analysis, potentially uncovering issues that may not be detected in the default normal mode.
		// However, this mode comes at the cost of longer execution times, making it suitable for scenarios where higher code quality is desired and longer waiting times are acceptable.
		// From version 2.14, the linter will prompt: "Limiting analysis of branches. Use --check-level=exhaustive to analyze all branches."
		a.LinterConfig.Args = append([]string{}, "--quiet", "--check-level=exhaustive", "--template='{file}:{line}:{column}: {message}'", ".")
	}

	return lint.GeneralHandler(ctx, log, a, lint.ExecRun, parser)
}

func parser(log *xlog.Logger, input []byte) (map[string][]lint.LinterOutput, []string) {
	lineParser := func(line string) (*lint.LinterOutput, error) {
		if len(line) <= 2 {
			return nil, nil
		}

		// remove the first and last character of the line,
		// which are the single quotes
		line = line[1 : len(line)-1]
		return lint.GeneralLineParser(line)
	}
	return lint.Parse(log, input, lineParser)
}
