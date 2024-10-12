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

package gomodcheck

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/xlog"
	"golang.org/x/mod/modfile"
)

var lintName = "gomodcheck"

func init() {
	linters.RegisterPullRequestHandler(lintName, goModCheckHandler)
	linters.RegisterLinterLanguages(lintName, []string{".go", ".mod"})
}

func goModCheckHandler(ctx context.Context, a linters.Agent) error {
	log := lintersutil.FromContext(ctx)
	parsedOutput, err := goModCheckOutput(log, a)
	if err != nil {
		log.Errorf("gomodchecks parse output failed: %v", err)
		return err
	}
	return linters.Report(ctx, a, parsedOutput)
}

func goModCheckOutput(log *xlog.Logger, a linters.Agent) (map[string][]linters.LinterOutput, error) {
	output := make(map[string][]linters.LinterOutput)
	for _, file := range a.Provider.GetFiles(nil) {
		fName := file
		if !strings.HasSuffix(fName, "go.mod") {
			continue
		}

		goModPath := filepath.Join(a.RepoDir, fName)
		file, err := os.ReadFile(goModPath)
		if err != nil {
			log.Errorf("Error opening %s: %s", goModPath, err)
			return output, err
		}

		mod, err := modfile.Parse("go.mod", file, nil)
		if err != nil {
			log.Errorf("Error parsing %s: %s", goModPath, err)
			return output, err
		}
		for _, replace := range mod.Replace {
			if !strings.HasPrefix(replace.New.Path, "../") {
				continue
			}

			parsePath := filepath.Join(filepath.Dir(goModPath), replace.New.Path)
			isSub, err := isSubdirectory(a.RepoDir, parsePath)
			if err != nil {
				log.Errorf("failed to compare whether A is a subdirectory of B : %v", err)
			}
			if !isSub {
				output[fName] = append(output[fName], linters.LinterOutput{
					File:    fName,
					Line:    replace.Syntax.Start.Line,
					Column:  replace.Syntax.Start.LineRune,
					Message: "cross-repository local replacement are not allowed[reviewbot]\nfor more information see https://github.com/qiniu/reviewbot/issues/275",
				})
			}
		}
	}

	return output, nil
}

// isSubdirectory reports whether the string b is subdirectory of a.
func isSubdirectory(a, b string) (bool, error) {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return false, err
	}

	return strings.HasPrefix(absB, absA), nil
}
