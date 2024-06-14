package stylecheck

import (
	"embed"
	"regexp"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

const linterName = "stylecheck"

//go:embed ruleset/*
var resources embed.FS
var rulePath string

func init() {
	linters.RegisterPullRequestHandler(linterName, stylecheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})
	rulePath = "/config/linters-config/.java-sun-checks.xml"
}

func stylecheckHandler(log *xlog.Logger, a linters.Agent) error {
	var javaFiles []string
	for _, arg := range a.PullRequestChangedFiles {
		if strings.HasSuffix(arg.GetFilename(), ".java") {
			javaFiles = append(javaFiles, arg.GetFilename())
		}
	}

	if len(javaFiles) > 0 {
		if linters.IsEmpty(a.LinterConfig.Args...) {
			args := append([]string{}, "-jar", "/usr/local/checkstyle-10.17.0-all.jar")
			args = append(args, javaFiles...)
			args = append(args, "-c", rulePath)
			a.LinterConfig.Args = args
			a.LinterConfig.Command = "java"
		}
	}

	return linters.GeneralHandler(log, a, func(l *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
		output = []byte(TrimReport(string(output)))
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

func TrimReport(line string) string {
	re := regexp.MustCompile("(?m)^.*检查.*$[\r\n]")
	reEnd := regexp.MustCompile("(?m)^.*错误结束.*$[\r\n]")
	line = re.ReplaceAllString(line, "")
	line = reEnd.ReplaceAllString(line, "")
	return line
}
