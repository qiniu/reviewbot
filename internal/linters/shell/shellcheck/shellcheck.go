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
	"context"
	"strings"

	"github.com/qiniu/reviewbot/internal/lint"
	"github.com/qiniu/reviewbot/internal/metric"
	"github.com/qiniu/reviewbot/internal/util"
)

// refer to https://github.com/koalaman/shellcheck
const linterName = "shellcheck"

func init() {
	lint.RegisterPullRequestHandler(linterName, shellcheck)
	lint.RegisterLinterLanguages(linterName, []string{".sh"})
}

func shellcheck(ctx context.Context, a lint.Agent) error {
	log := util.FromContext(ctx)
	var shellFiles []string
	for _, arg := range a.Provider.GetFiles(nil) {
		if strings.HasSuffix(arg, ".sh") {
			shellFiles = append(shellFiles, arg)
		}
	}

	var lintResults map[string][]lint.LinterOutput
	if len(shellFiles) > 0 {
		cmd := a.LinterConfig.Command
		// execute shellcheck with the following command
		// shellcheck -f gcc xxx.sh...
		if lint.IsEmpty(a.LinterConfig.Args...) {
			// use gcc format to make the output more readable
			args := append([]string{}, "-f", "gcc")
			args = append(args, shellFiles...)
			a.LinterConfig.Args = args
		}

		output, err := lint.ExecRun(ctx, a)
		if err != nil {
			log.Warnf("%s run with error: %v, mark and continue", cmd, err)
		}

		results, unexpected := lint.GeneralParse(log, output)
		if len(unexpected) > 0 {
			msg := util.LimitJoin(unexpected, 1000)
			log.Warnf("unexpected output: %v", msg)
			metric.NotifyWebhookByText(lint.ConstructUnknownMsg(linterName, a.Provider.GetCodeReviewInfo().Org+"/"+a.Provider.GetCodeReviewInfo().Repo, a.Provider.GetCodeReviewInfo().URL, log.ReqId, msg))
		}

		lintResults = results
	}

	// even if the lintResults is empty, we still need to report the result
	// since we need delete the existed comments related to the linter
	return lint.Report(ctx, a, lintResults)
}
