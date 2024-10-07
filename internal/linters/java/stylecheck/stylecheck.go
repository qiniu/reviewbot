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

package stylecheck

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/errors"
	"github.com/qiniu/x/xlog"
)

const linterName = "stylecheck"

const (
	styleJarURL   = "https://github.com/checkstyle/checkstyle/releases/download/checkstyle-10.17.0/checkstyle-10.17.0-all.jar"
	localStyleJar = "/checkstyle.jar"
	styleRuleURL  = "https://raw.githubusercontent.com/checkstyle/checkstyle/master/src/main/resources/sun_checks.xml"
	styleRuleDir  = "/var/tmp/linters-config/"
)

func init() {
	linters.RegisterPullRequestHandler(linterName, stylecheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})
}

func stylecheckHandler(ctx context.Context, a linters.Agent) error {
	slog := lintersutil.FromContext(ctx)
	var javaFiles []string
	rulePath := a.LinterConfig.ConfigPath
	for _, arg := range a.PullRequestChangedFiles {
		if strings.HasSuffix(arg.GetFilename(), ".java") {
			javaFiles = append(javaFiles, arg.GetFilename())
		}
	}
	if len(javaFiles) == 0 {
		return nil
	}
	checkrulePath, checkerr := styleRuleCheck(a.LinterConfig.WorkDir)(slog, rulePath)
	if checkerr != nil {
		slog.Errorf("style rule check failed: %v", checkerr)
		return checkerr
	}

	a = argsApply(slog, a)
	a.LinterConfig.Args = append(a.LinterConfig.Args, "-jar", localStyleJar, "-c", checkrulePath)
	a.LinterConfig.Args = append(a.LinterConfig.Args, javaFiles...)

	return linters.GeneralHandler(ctx, slog, a, linters.ExecRun, stylecheckParser(a.LinterConfig.WorkDir))
}

func argsApply(log *xlog.Logger, a linters.Agent) linters.Agent {
	config := a.LinterConfig
	if len(a.LinterConfig.Command) == 1 && a.LinterConfig.Command[0] == linterName {
		config.Command = []string{"java"}
	}
	log.Info("stylecheck comamnd:" + strings.Join(config.Command, " "))
	if linters.IsEmpty(config.Args...) {
		args := append([]string{}, "")
		config.Args = args
	}
	a.LinterConfig = config
	return a
}

func stylecheckParser(codedir string) func(slog *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
	return func(slog *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
		lineParse := func(line string) (*linters.LinterOutput, error) {
			// stylecheck will output lines starting with ' 开始检查(Starting audit) ' or '检查结束(Audit done) ' or 'stylecheck result(Checkstyle ends with 20 errors.)'
			// which are no meaningful for the reviewbot scenario, so we discard them Starting audit done.
			if strings.Contains(strings.ToLower(line), "checkstyle") || strings.HasPrefix(line, "Starting audit") || strings.HasPrefix(line, "Audit done") || strings.HasPrefix(line, "检查") {
				return nil, nil
			}
			line = strings.ReplaceAll(line, "[ERROR]", "")
			line = strings.ReplaceAll(line, codedir+"/", "")
			return linters.GeneralLineParser(strings.TrimLeft(line, " "))
		}
		return linters.Parse(slog, output, lineParse)
	}
}

func stylecheckJar(slog *xlog.Logger) (string, error) {
	jarfilepath := filepath.Join(styleRuleDir, localStyleJar)
	_, exist := lintersutil.FileExists(jarfilepath)
	if !exist {
		res, err := getFileFromURL(slog, styleJarURL)
		if err != nil {
			return "", err
		}
		return res, nil
	}
	return jarfilepath, nil
}

func getFileFromURL(slog *xlog.Logger, url string) (string, error) {
	newfile := filepath.Join(styleRuleDir, filepath.Base(url))
	res, err := http.Get(url)
	if err != nil {
		return "", err
	}
	merr := os.MkdirAll(styleRuleDir, os.ModePerm)
	if merr != nil {
		return "", merr
	}
	f, err := os.Create(newfile)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(f, res.Body)
	defer res.Body.Close()
	if err != nil {
		slog.Errorf("the file saving   encountered an error: %v", err)
		return "", err
	}
	return newfile, nil
}

func styleRuleCheck(codedir string) func(slog *xlog.Logger, styleConf string) (string, error) {
	return func(slog *xlog.Logger, styleConf string) (string, error) {
		tmpnewfile := filepath.Join(styleRuleDir, "tmp", filepath.Base(styleRuleURL))
		if styleConf == "" {
			absfilepath, _ := lintersutil.FileExists(tmpnewfile)
			if absfilepath != "" {
				return absfilepath, nil
			}
			downloadfilepath, err := getFileFromURL(slog, styleRuleURL)
			if err != nil {
				slog.Errorf("the style rule file download faild: %v", err)
				return "", err
			}
			return downloadfilepath, nil
		}
		if strings.HasPrefix(styleConf, "http://") || strings.HasPrefix(styleConf, "https://") {
			downloadfilepath, err := getFileFromURL(slog, styleRuleURL)
			if err != nil {
				slog.Errorf("the style rule file download faild: %v", err)
				return "", err
			}
			return downloadfilepath, nil
		}
		absfilepath, exist := lintersutil.FileExists(styleConf)
		if exist {
			return absfilepath, nil
		}
		rulefilepathcode := filepath.Join(codedir, styleConf)
		absfilepathcode, existcode := lintersutil.FileExists(rulefilepathcode)
		if existcode {
			return absfilepathcode, nil
		}
		return "", errors.New("the style rule file not exist")
	}
}
