package pmdcheck

import (
	"github.com/qiniu/x/log"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

// refer to https://pmd.github.io/
const linterName = "pmdcheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, pmdcheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})

}

func pmdcheckHandler(log *xlog.Logger, a linters.Agent) error {
	var javaFiles []string
	rulePath := a.LinterConfig.ConfigPath
	for _, arg := range a.PullRequestChangedFiles {
		if strings.HasSuffix(arg.GetFilename(), ".java") {
			javaFiles = append(javaFiles, a.LinterConfig.WorkDir+"/"+arg.GetFilename())
		}
	}
	checkrulePath, checkerr := pmdRuleCheck(rulePath)
	if checkerr != nil {
		log.Errorf("pmd rule file check failed: %v", checkerr)
	}
	log.Infof("pmd  rule check succes,file path: %v", checkrulePath)
	if (len(javaFiles) <= 0) || !linters.IsExist(checkrulePath) || checkerr != nil {
		return nil
	}
	if linters.IsEmpty(a.LinterConfig.Args...) {
		args := append([]string{}, "check")
		args = append(args, "-f", "emacs")
		args = append(args, javaFiles...)
		args = append(args, "-R", checkrulePath)
		a.LinterConfig.Args = args
	}
	if a.LinterConfig.Command == "" || a.LinterConfig.Command == linterName {
		a.LinterConfig.Command = "pmd"
	}
	if a.LinterConfig.LinterName == "" {
		a.LinterConfig.LinterName = linterName
	}
	return linters.GeneralHandler(log, a, pmdcheckParser)

}
func pmdcheckParser(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	var lineParse = func(line string) (*linters.LinterOutput, error) {
		// pmdcheck will output lines starting with ' [WARN]'  warring information
		// which are no meaningful for the reviewbot scenario, so we discard them

		strings.ToLower(line)
		if strings.Contains(line, "[WARN]") || line == "" {
			return nil, nil
		}
		return linters.GeneralLineParser(strings.TrimLeft(line, " "))
	}
	return linters.Parse(log, output, lineParse)
}

func getFileFromURl(url string, filepath string) (string, error) {
	if linters.IsExist(filepath) {
		return filepath, nil
	}
	res, err := http.Get(url)
	if err != nil {
		log.Errorf("The file download  encountered  an error，Please check the file  download url: %v,the error is:%v", url, err)
		return "", err
	}

	f, err := os.Create(filepath)
	if err != nil {
		log.Errorf("The file saving   encountered an error,Please check the directory: %v", err)
		return "", err
	}
	_, err = io.Copy(f, res.Body)
	defer res.Body.Close()

	if err != nil {
		log.Errorf("The file saving   encountered an error: %v", err)
		return "", err
	}
	if linters.IsExist(filepath) {
		log.Infof("pmd  rule check succes,file path: %v", filepath)
		return filepath, nil
	}
	return "", err
}
func pmdRuleCheck(pmdConf string) (string, error) {
	if linters.IsExist(pmdConf) {
		return pmdConf, nil

	}
	if pmdConf == "" {
		pmdConf = "https://raw.githubusercontent.com/pmd/pmd/master/pmd-java/src/main/resources/category/java/bestpractices.xml"
	}
	fileDir, err := os.Getwd()
	rulefiledirpath := filepath.Join(fileDir, "config/linters-config")
	rulefilepath := filepath.Join(rulefiledirpath, ".bestpractices.xml")
	madirerr := os.MkdirAll(rulefiledirpath, 0755)
	if madirerr != nil {
		log.Errorf("dir make failed: %v", err)
		return "", err
	}
	if strings.HasPrefix(pmdConf, "http") {
		downloadfilepath, err := getFileFromURl(pmdConf, rulefilepath)
		if err != nil {
			log.Errorf("the pmd rule file download faild: %v", err)
			return "", err
		}
		return downloadfilepath, nil
	}
	log.Errorf("the pmd rule file not exist: %v", err)
	return "", err
}