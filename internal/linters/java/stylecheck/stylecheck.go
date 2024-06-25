package stylecheck

import (
	"fmt"
	"github.com/qiniu/x/log"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

const linterName = "stylecheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, stylecheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})

}

func stylecheckHandler(log *xlog.Logger, a linters.Agent) error {
	var javaFiles []string
	rulePath := a.LinterConfig.ConfigPath
	for _, arg := range a.PullRequestChangedFiles {
		if strings.HasSuffix(arg.GetFilename(), ".java") {
			javaFiles = append(javaFiles, a.LinterConfig.WorkDir+"/"+arg.GetFilename())
		}
	}
	jarfile, err := stylecheckJar()
	if err != nil {
		log.Errorf("style jar check failed: %v", err)
	}
	log.Infof("sytle jar check succes,file path: %v", jarfile)
	checkrulePath, checkerr := styleRuleCheck(rulePath)
	if checkerr != nil {
		log.Errorf("style rule file check failed: %v", checkerr)
	}
	log.Infof("sytle  rule check succes,file path: %v", checkrulePath)

	if (len(javaFiles) <= 0) || !linters.IsExist(checkrulePath) || linters.IsExist(jarfile) && err != nil || checkerr != nil {
		return nil
	}
	if linters.IsEmpty(a.LinterConfig.Args...) {
		args := append([]string{}, "-jar", jarfile)
		args = append(args, javaFiles...)
		args = append(args, "-c", checkrulePath)
		a.LinterConfig.Args = args
	}
	if a.LinterConfig.Command == "" || a.LinterConfig.Command == linterName {
		a.LinterConfig.Command = "java"
	}
	if a.LinterConfig.LinterName == "" {
		a.LinterConfig.LinterName = linterName
	}

	return linters.GeneralHandler(log, a, stylecheckParser)
}
func stylecheckParser(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	var lineParse = func(line string) (*linters.LinterOutput, error) {
		// stylecheck will output lines starting with ' 开始检查 ' or '检查结束 ' or 'stylecheck info'
		// which are no meaningful for the reviewbot scenario, so we discard them
		strings.ToLower(line)
		if strings.Contains(strings.ToLower(line), "checkstyle") || strings.HasPrefix(line, "开始") || strings.HasPrefix(line, "检查") || line == "" {
			return nil, nil
		}
		return linters.GeneralLineParser(strings.TrimLeft(line, " "))
	}
	return linters.Parse(log, output, lineParse)
}

func stylecheckJar() (string, error) {
	var stylejar = "/usr/local/checkstyle-10.17.0-all.jar"
	if linters.IsExist(stylejar) {
		return stylejar, nil

	}
	var stylejarurl = "https://github.com/checkstyle/checkstyle/releases/download/checkstyle-10.17.0/checkstyle-10.17.0-all.jar"
	var stykejarfilename = "checkstyle-10.17.0-all.jar"
	filePath, err := os.Getwd()
	if err != nil {
		log.Errorf("get work dir failed: %v", err)
		return "", err
	}
	filename2 := filepath.Join(filePath, stykejarfilename)
	fmt.Println(filename2)
	if linters.IsExist(filename2) {
		return filename2, nil
	}
	res, err := http.Get(stylejarurl)
	if err != nil {
		return "", fmt.Errorf("The file download  encountered  an error，Please check the file  download url: %v", err)
	}
	madirerr := os.MkdirAll(filePath, 0755)
	if madirerr != nil {
		return "", madirerr
	}
	f, err := os.Create(filename2)
	if err != nil {
		fmt.Println(f, err)
		return "", fmt.Errorf("The file saving   encountered an error,Please check the directory: %v", err)
	}
	_, err = io.Copy(f, res.Body)
	defer res.Body.Close()

	if err != nil {
		return "", fmt.Errorf("The file saving   encountered an error: %v", err)
	}
	if linters.IsExist(filename2) {

		fmt.Printf("style jar download success : %v", err)
		return filename2, nil
	}
	return "", fmt.Errorf("The style jar file download  encountered  an error")

}
func getFileFromUrl(url string, filepath string) (string, error) {
	if linters.IsExist(filepath) {
		return filepath, nil
	}
	res, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("The file download  encountered  an error，Please check the file  download url: %v,the error is:%v", url, err)
	}

	f, err := os.Create(filepath)
	if err != nil {

		return "", fmt.Errorf("The file saving   encountered an error,Please check the directory: %v", err)
	}
	_, err = io.Copy(f, res.Body)
	defer res.Body.Close()

	if err != nil {
		return "", fmt.Errorf("The file saving   encountered an error: %v", err)
	}
	if linters.IsExist(filepath) {
		log.Infof("style  rule check succes,file path: %v", filepath)
		return filepath, nil
	}
	return "", err
}
func styleRuleCheck(styleConf string) (string, error) {
	if linters.IsExist(styleConf) {
		return styleConf, nil

	}
	if styleConf == "" {
		styleConf = "https://raw.githubusercontent.com/checkstyle/checkstyle/master/src/main/resources/sun_checks.xml"
	}
	fileDir, err := os.Getwd()
	rulefiledirpath := filepath.Join(fileDir, "config/linters-config")
	rulefilepath := filepath.Join(rulefiledirpath, ".sun_checks.xml")
	madirerr := os.MkdirAll(rulefiledirpath, 0755)
	if madirerr != nil {
		return "", fmt.Errorf("dir make failed: %v", err)
	}
	if strings.HasPrefix(styleConf, "http") {
		downloadfilepath, err := getFileFromUrl(styleConf, rulefilepath)
		if err != nil {
			return "", fmt.Errorf("the style rule file download faild: %v", err)
		}
		return downloadfilepath, nil
	}
	return "", fmt.Errorf("the style rule file not exist: %v", err)
}
