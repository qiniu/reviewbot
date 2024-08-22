package pmdcheck

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/errors"
	"github.com/qiniu/x/xlog"
)

// refer to https://pmd.github.io/
const linterName = "pmdcheck"
const pmdRuleURL = "https://raw.githubusercontent.com/pmd/pmd/master/pmd-java/src/main/resources/category/java/bestpractices.xml"
const pmdRuleDir = "/var/tmp/linters-config/"

func init() {
	linters.RegisterPullRequestHandler(linterName, pmdCheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})
}

func pmdCheckHandler(plog *xlog.Logger, a linters.Agent) error {
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
	checkrulePath, checkerr := pmdRuleCheck(plog, rulePath, a)
	if checkerr != nil {
		plog.Errorf("pmd rule check failed: %v", checkerr)
		return checkerr
	}

	if linters.IsEmpty(a.LinterConfig.Args...) {
		args := append([]string{}, "check")
		args = append(args, "-f", "emacs")
		args = append(args, javaFiles...)
		args = append(args, "-R", checkrulePath)
		a.LinterConfig.Args = args
	}
	if reflect.DeepEqual(a.LinterConfig.Command, []string{linterName}) {
		a.LinterConfig.Command = []string{"pmd"}
	}
	return linters.GeneralHandler(plog, a, linters.ExecRun, pmdcheckParser)
}

func pmdcheckParser(plog *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
	var lineParse = func(line string) (*linters.LinterOutput, error) {
		// pmdcheck will output lines starting with ' [WARN]' or '[ERROR]'  warring/error information
		// which are no meaningful for the reviewbot scenario, so we discard them
		if strings.Contains(line, "[WARN]") || strings.Contains(line, "[ERROR]") {
			return nil, nil
		}
		return linters.GeneralLineParser(strings.TrimLeft(line, " "))
	}
	return linters.Parse(plog, output, lineParse)
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

func pmdRuleCheck(plog *xlog.Logger, pmdConf string, a linters.Agent) (string, error) {
	tmpnewfile := filepath.Join(pmdRuleDir, filepath.Base(pmdRuleURL))
	if pmdConf == "" {
		absfilepath, _ := lintersutil.FileExists(tmpnewfile)
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
	absfilepath, exist := lintersutil.FileExists(pmdConf)
	if exist {
		return absfilepath, nil
	}
	pmdconfpath := filepath.Join(a.LinterConfig.WorkDir, pmdConf)
	abspmdfilepath, _ := lintersutil.FileExists(pmdconfpath)
	if absfilepath != "" {
		return abspmdfilepath, nil
	}
	return "", errors.New("the pmd rule file not exist")
}
