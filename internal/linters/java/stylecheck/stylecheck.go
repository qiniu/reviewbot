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
	if (len(javaFiles) <= 0) || !linters.IsExist(rulePath) || linters.IsExist(jarfile) && err != nil {
		return nil
	}

	if linters.IsEmpty(a.LinterConfig.Args...) {
		args := append([]string{}, "-jar", jarfile)
		args = append(args, javaFiles...)
		args = append(args, "-c", rulePath)
		a.LinterConfig.Args = args
	}
	if a.LinterConfig.Command == "" || a.LinterConfig.Command == linterName {
		a.LinterConfig.Command = "java"
	}
	if a.LinterConfig.LinterName == "" {
		a.LinterConfig.LinterName = linterName
	}

	return linters.GeneralHandler(log, a, func(l *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
		//output = []byte(trimReport(string(output)))
		return linters.Parse(log, output, stylecheckParser)
	})
}

func stylecheckParser(line string) (*linters.LinterOutput, error) {
	if strings.EqualFold(line, "checkstyle") {
		return nil, nil
	}
	lineResult, err := linters.GeneralLineParser(line)
	if err != nil {
		return nil, err

	}
	return &linters.LinterOutput{
		File:    strings.TrimLeft(lineResult.File, " "),
		Line:    lineResult.Line,
		Column:  lineResult.Column,
		Message: strings.ReplaceAll(strings.ReplaceAll(lineResult.Message, "\x1b[0m\x1b[1m", ""), "\x1b[0m", ""),
	}, nil
}

func trimReport(line string) string {
	line1st := strings.ReplaceAll(line, "开始检查……\n", "&stylecheck&")
	line2nd := strings.ReplaceAll(line1st, "\n检查完成", "&stylecheck&")
	lineresult := strings.Split(line2nd, "&stylecheck&")
	if len(lineresult) < 3 {
		return ""
	}
	return lineresult[1]

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
		return filename2, nil
	}
	return "", fmt.Errorf("The style jar file download  encountered  an error")

}
