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

	"github.com/qiniu/reviewbot/internal/lint"
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
	extensions := []string{".sh", ".bash", ".ksh", ".zsh"}
	shellFiles, err := util.FindFileWithExt(a.RepoDir, extensions)
	if err != nil {
		log.Errorf("Error walking the path: %v", err)
		return err
	}

	if a.CLIMode && len(shellFiles) == 0 {
		log.Infof("CLI-info:skip to run shellcheck")
		return nil
	}

	if len(shellFiles) > 0 {
		// execute shellcheck with the following command
		// shellcheck -f gcc xxx.sh...
		if lint.IsEmpty(a.LinterConfig.Args...) {
			// use gcc format to make the output more readable
			args := append([]string{}, "-f", "gcc")
			args = append(args, shellFiles...)
			a.LinterConfig.Args = args
		}
	}
	return lint.GeneralHandler(ctx, log, a, lint.ExecRun, lint.GeneralParse)
}
