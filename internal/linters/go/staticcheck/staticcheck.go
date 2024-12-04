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

// Deprecated. use golangci-lint instead
package staticcheck

import (
	"context"

	"github.com/qiniu/reviewbot/internal/lint"
	"github.com/qiniu/reviewbot/internal/util"
)

// refer to https://staticcheck.io/docs/
const linterName = "staticcheck"

func init() {
	lint.RegisterPullRequestHandler(linterName, staticcheckHandler)
	lint.RegisterLinterLanguages(linterName, []string{".go"})
}

func staticcheckHandler(ctx context.Context, a lint.Agent) error {
	log := util.FromContext(ctx)
	if lint.IsEmpty(a.LinterConfig.Args...) {
		// turn off compile errors by default
		a.LinterConfig.Args = append([]string{}, "-debug.no-compile-errors=true", "./...")
	}

	return lint.GeneralHandler(ctx, log, a, lint.ExecRun, lint.GeneralParse)
}
