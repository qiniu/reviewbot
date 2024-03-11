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

package staticcheck

import (
	"context"
	"fmt"
	"regexp"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

// refer to https://staticcheck.io/docs/
const linterName = "staticcheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, staticcheckHandler)
}

func staticcheckHandler(log *xlog.Logger, a linters.Agent) error {
	if linters.IsEmpty(a.LinterConfig.Args...) {
		// turn off compile errors by default
		a.LinterConfig.Args = append([]string{}, "-debug.no-compile-errors=true", "./...")
	}

	ctx := context.Background()
	query := fmt.Sprintf("repo:%s/%s filename:go.mod github.com/zeromicro/go-zero",
		a.PullRequestEvent.Repo.GetOwner().GetLogin(),
		a.PullRequestEvent.Repo.GetName())
	result, _, err := a.GithubClient.Search.Code(ctx, query, nil)
	if err != nil {
		log.Errorf("Error searching code: %v\n", err)
	} else {
		if *result.Total > 0 {
			log.Info("add gozero filter")
			a.OutputFilters = append(a.OutputFilters, &defaultGozeroFilter)
		}
	}
	return linters.GeneralHandler(log, a, linters.GeneralParse)
}

var defaultGozeroFilter = GozeroOutputFilter{
	regex:   `unknown JSON option "([a-z]+).+"`,
	options: []string{"optional", "options", "default", "range"},
}

type GozeroOutputFilter struct {
	regex   string
	options []string
}

func (f *GozeroOutputFilter) Filter(o *linters.LinterOutput) *linters.LinterOutput {
	if compile, err := regexp.Compile(f.regex); err == nil {
		optionStrs := compile.FindStringSubmatch(o.Message)
		if len(optionStrs) >= 2 {
			for _, option := range f.options {
				if option == optionStrs[1] {
					return nil
				}
			}
		}
	}

	return o
}
