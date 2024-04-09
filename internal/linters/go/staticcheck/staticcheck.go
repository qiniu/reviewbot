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
	"regexp"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

// refer to https://staticcheck.io/docs/
const linterName = "staticcheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, staticcheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".go"})
}

func staticcheckHandler(log *xlog.Logger, a linters.Agent) error {
	if linters.IsEmpty(a.LinterConfig.Args...) {
		// turn off compile errors by default
		a.LinterConfig.Args = append([]string{}, "-debug.no-compile-errors=true", "./...")
	}

	return linters.GeneralHandler(log, a, staticcheckParse)
}

// staticcheckParse parses the output of staticcheck.
func staticcheckParse(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	results, err := linters.GeneralParse(log, output)
	if err != nil {
		return nil, err
	}

	finalResults := make(map[string][]linters.LinterOutput)

	// special handling for SA5008 (unknown JSON option error)
	// Background:
	//  * https://github.com/qiniu/reviewbot/issues/24
	tagRex := regexp.MustCompile(`unknown JSON option "(.*)" \(SA5008\)`)
	for file, linterResults := range results {
		var lintersCopy []linters.LinterOutput
		for _, linter := range linterResults {
			matches := tagRex.FindStringSubmatch(linter.Message)
			if len(matches) == 2 && isGoZeroCustomTag(matches[1]) {
				log.Warnf("ignore this error: %v", linter.Message)
				continue
			}
			lintersCopy = append(lintersCopy, linter)
		}
		if len(lintersCopy) > 0 {
			finalResults[file] = lintersCopy
		}
	}

	return finalResults, nil
}

// isGoZeroCustomTag checks if the tag is a go-zero custom tag.
// refer: https://go-zero.dev/en/docs/tutorials/go-zero/configuration/overview#tag-checksum-rule
const (
	goZeroDefaultOption  = "default"
	goZeroEnvOption      = "env"
	goZeroInheritOption  = "inherit"
	goZeroOptionalOption = "optional"
	goZeroOptionsOption  = "options"
	goZeroRangeOption    = "range"
)

// FIXME(CarlJi): this function is a temporary solution for go-zero custom tag, see [#24](https://github.com/qiniu/reviewbot/issues/24)
// expect to remove this function after staticcheck supports go-zero custom tag.
func isGoZeroCustomTag(jsonOption string) bool {
	var found bool
	switch {
	case strings.HasPrefix(jsonOption, goZeroDefaultOption):
		found = true
	case strings.HasPrefix(jsonOption, goZeroEnvOption):
		found = true
	case strings.HasPrefix(jsonOption, goZeroInheritOption):
		found = true
	case strings.HasPrefix(jsonOption, goZeroOptionalOption):
		found = true
	case strings.HasPrefix(jsonOption, goZeroOptionsOption):
		found = true
	case strings.HasPrefix(jsonOption, goZeroRangeOption):
		found = true
	}

	if found {
		log.Warnf("this tag %v seems belongs to go-zero, ignore it temporary, see [#24](https://github.com/qiniu/reviewbot/issues/24) for more information", jsonOption)
	}

	return found
}
