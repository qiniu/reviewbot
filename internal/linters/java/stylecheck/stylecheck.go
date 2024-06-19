package stylecheck

import (
	"fmt"
	"github.com/qiniu/x/log"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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
	if (len(javaFiles) > 0) && linters.IsExist(rulePath) && linters.IsEmpty(a.LinterConfig.Args...) && err == nil {
		stylecheckJar()
		//args := append([]string{}, "-jar", "/usr/local/checkstyle-10.17.0-all.jar")
		args := append([]string{}, "-jar", jarfile)
		args = append(args, javaFiles...)
		args = append(args, "-c", rulePath)
		a.LinterConfig.Args = args
		a.LinterConfig.Command = "java"
		a.LinterConfig.LinterName = "stylecheck"

	}

	return linters.GeneralHandler(log, a, func(l *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
		output = []byte(trimReport(string(output)))
		return linters.Parse(log, output, stylecheckParser)
	})
}

func stylecheckParser(line string) (*linters.LinterOutput, error) {
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
	pattern := `(.*?)开始检查……\n([\d\D]*)\n检查完成([\d\D]*)?`
	//pattern := `^(.*?)Running([\d\D ]*)27([\d\D]*)?$`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		log.Errorf("compile regex failed: %v", err)
		return ""
	}
	matches := regex.FindStringSubmatch(line)
	if len(matches) > 3 || matches != nil {
		return matches[2]
	} else {
		return ""
	}
}

func stylecheckJar() (string, error) {
	var stykejar = "/usr/local/checkstyle-10.17.0-all.jar"
	if linters.IsExist(stykejar) {
		return stykejar, nil

	}
	var stylejarurl = "https://github.com/checkstyle/checkstyle/releases/download/checkstyle-10.17.0/checkstyle-10.17.0-all.jar"
	var stykejarfilename = "checkstyle-10.17.0-all.jar"
	res, err := http.Get(stylejarurl)
	if err != nil {
		return "", fmt.Errorf("The file download  encountered  an error，Please check the file  download url: %v", err)
	}
	filePath, err := os.Getwd()
	if err != nil {
		log.Errorf("get work dir failed: %v", err)
		return "", err
	}
	filename2 := filepath.Join(filePath, stykejarfilename)
	if linters.IsExist(filename2) {
		return filename2, nil
	}
	os.MkdirAll(filePath, 0755)
	f, err := os.Create(filename2)
	if err != nil {
		fmt.Println(f, err)
		return "", fmt.Errorf("The file saving   encountered an error,Please check the directory: %v", err)
	}
	_, err = io.Copy(f, res.Body)
	if err != nil {
		return "", fmt.Errorf("The file saving   encountered an error: %v", err)
	}
	if linters.IsExist(filename2) {
		return filename2, nil
	} else {
		return "", fmt.Errorf("The style jar file download  encountered  an error")
	}

}