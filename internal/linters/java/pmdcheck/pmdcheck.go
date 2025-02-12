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

package pmdcheck

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/qiniu/reviewbot/internal/lint"
	"github.com/qiniu/reviewbot/internal/util"
	"github.com/qiniu/x/errors"
	"github.com/qiniu/x/xlog"
)

// refer to https://pmd.github.io/
const (
	linterName = "pmdcheck"
	pmdRuleURL = "https://raw.githubusercontent.com/pmd/pmd/master/pmd-java/src/main/resources/category/java/bestpractices.xml"
	pmdRuleDir = "/var/tmp/linters-config/"
)

func init() {
	lint.RegisterPullRequestHandler(linterName, pmdCheckHandler)
	lint.RegisterLinterLanguages(linterName, []string{".java"})
}

func pmdCheckHandler(ctx context.Context, a lint.Agent) error {
	plog := util.FromContext(ctx)
	var javaFiles []string
	rulePath := a.LinterConfig.ConfigPath
	javaFiles, err := util.FindFileWithExt(a.RepoDir, []string{".java"})
	if err != nil {
		return err
	}

	if len(javaFiles) == 0 {
		return nil
	}
	checkrulePath, checkerr := pmdRuleCheck(plog, rulePath, a)
	if checkerr != nil {
		plog.Errorf("pmd rule check failed: %v", checkerr)
		return checkerr
	}
	a = argsApply(plog, a)
	a.LinterConfig.Args = append(append(a.LinterConfig.Args, javaFiles...), "-R", checkrulePath)
	return lint.GeneralHandler(ctx, plog, a, lint.ExecRun, pmdcheckParser)
}

func argsApply(log *xlog.Logger, a lint.Agent) lint.Agent {
	config := a.LinterConfig
	if len(config.Command) == 1 && config.Command[0] == linterName {
		config.Command = []string{"pmd"}
	}
	log.Info("pmdcheck comamnd:" + strings.Join(config.Command, " "))
	if lint.IsEmpty(config.Args...) {
		args := append([]string{}, "check")
		args = append(args, "-f", "emacs")
		config.Args = args
	}
	a.LinterConfig = config
	return a
}

func pmdcheckParser(plog *xlog.Logger, output []byte) (map[string][]lint.LinterOutput, []string) {
	lineParse := func(line string) (*lint.LinterOutput, error) {
		// pmdcheck will output lines starting with ' [WARN]' or '[ERROR]'  warring/error information
		// which are no meaningful for the reviewbot scenario, so we discard them
		if strings.Contains(line, "[WARN]") || strings.Contains(line, "[ERROR]") {
			return nil, nil
		}
		return lint.GeneralLineParser(strings.TrimLeft(line, " "))
	}
	return lint.Parse(plog, output, lineParse)
}

func getFileFromURL(plog *xlog.Logger, url string) (string, error) {
	newfile := filepath.Join(pmdRuleDir, filepath.Base(url))
	res, err := http.Get(url)
	if err != nil {
		plog.Errorf("the file download encountered an error, Please check the file download url: %v, the error is:%v", url, err)
		return "", err
	}
	if err := os.MkdirAll(pmdRuleDir, os.ModePerm); err != nil {
		plog.Fatalf("failed to  create check rule config dir: %v", err)
	}
	f, err := os.Create(newfile)
	if err != nil {
		plog.Errorf("the file saving encountered an error, Please check the directory: %v", err)
		return "", err
	}
	_, err = io.Copy(f, res.Body)
	defer res.Body.Close()
	if err != nil {
		plog.Errorf("the file saving   encountered an error: %v", err)
		return "", err
	}
	return newfile, nil
}

func pmdRuleCheck(plog *xlog.Logger, pmdConf string, a lint.Agent) (string, error) {
	tmpnewfile := filepath.Join(pmdRuleDir, filepath.Base(pmdRuleURL))
	if pmdConf == "" {
		absfilepath, _ := util.FileExists(tmpnewfile)
		if absfilepath != "" {
			return absfilepath, nil
		}
		downloadfilepath, err := getFileFromURL(plog, pmdRuleURL)
		if err != nil {
			plog.Errorf("the pmd rule file download faild: %v", err)
			return "", err
		}
		return downloadfilepath, nil
	}
	if strings.HasPrefix(pmdConf, "http://") || strings.HasPrefix(pmdConf, "https://") {
		downloadfilepath, err := getFileFromURL(plog, pmdRuleURL)
		if err != nil {
			plog.Errorf("the pmd rule file download faild: %v", err)
			return "", err
		}
		return downloadfilepath, nil
	}
	absfilepath, exist := util.FileExists(pmdConf)
	if exist {
		return absfilepath, nil
	}
	pmdconfpath := filepath.Join(a.LinterConfig.WorkDir, pmdConf)
	abspmdfilepath, _ := util.FileExists(pmdconfpath)
	if absfilepath != "" {
		return abspmdfilepath, nil
	}
	return "", errors.New("the pmd rule file not exist")
}
