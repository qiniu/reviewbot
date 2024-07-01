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
package shellcheck

import (
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/reviewbot/internal/metric"
	"github.com/qiniu/x/xlog"
)

// refer to https://github.com/koalaman/shellcheck
const linterName = "shellcheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, shellcheck)
	linters.RegisterLinterLanguages(linterName, []string{".sh"})
}

func shellcheck(log *xlog.Logger, a linters.Agent) error {
	var shellFiles []string
	for _, arg := range a.PullRequestChangedFiles {
		if strings.HasSuffix(arg.GetFilename(), ".sh") {
			shellFiles = append(shellFiles, arg.GetFilename())
		}
	}

	var lintResults map[string][]linters.LinterOutput
	if len(shellFiles) > 0 {
		cmd := a.LinterConfig.Command
		// execute shellcheck with the following command
		// shellcheck -f gcc xxx.sh...
		if linters.IsEmpty(a.LinterConfig.Args...) {
			// use gcc format to make the output more readable
			args := append([]string{}, "-f", "gcc")
			args = append(args, shellFiles...)
			a.LinterConfig.Args = args
		}

		output, err := linters.ExecRun(a.LinterConfig.WorkDir, a.LinterConfig.Command, a.LinterConfig.Args)
		if err != nil {
			log.Warnf("%s run with error: %v, mark and continue", cmd, err)
		}

		results, unexpected := linters.GeneralParse(log, output)
		if len(unexpected) > 0 {
			msg := lintersutil.LimitJoin(unexpected, 1000)
			log.Warnf("unexpected output: %v", msg)
			metric.NotifyWebhookByText(linters.ConstructUnknownMsg(linterName, a.PullRequestEvent.Repo.GetFullName(), a.PullRequestEvent.PullRequest.GetHTMLURL(), log.ReqId, msg))
		}

		lintResults = results
	}

	// even if the lintResults is empty, we still need to report the result
	// since we need delete the existed comments related to the linter
	return linters.Report(log, a, lintResults)
}
