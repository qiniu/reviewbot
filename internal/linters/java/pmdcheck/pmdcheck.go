package pmdcheck

import (
	"regexp"
	"strings"

	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

// refer to https://pmd.github.io/
const linterName = "pmdcheck"

var rulePath string

func init() {
	linters.RegisterPullRequestHandler(linterName, pmdcheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})
	var c config.GlobalConfig
	rulePath = c.JavaPmdCheckRuleConfig

}

func pmdcheckHandler(log *xlog.Logger, a linters.Agent) error {
	var javaFiles []string
	for _, arg := range a.PullRequestChangedFiles {
		if strings.HasSuffix(arg.GetFilename(), ".java") {
			javaFiles = append(javaFiles, arg.GetFilename())
		}
	}

	if len(javaFiles) > 0 {
		if linters.IsEmpty(a.LinterConfig.Args...) {
			args := append([]string{}, "check")
			args = append(args, "-f", "emacs")
			args = append(args, javaFiles...)
			args = append(args, "-R", rulePath)
			a.LinterConfig.Args = args
			a.LinterConfig.Command = "pmd"
		}
	}

	return linters.GeneralHandler(log, a, func(l *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
		output = []byte(trimReport(string(output)))
		return linters.Parse(log, output, pmdcheckParser)
	})
}

func pmdcheckParser(line string) (*linters.LinterOutput, error) {
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
	re := regexp.MustCompile("(?m)^.*WARN.*$[\r\n]")
	line = re.ReplaceAllString(line, "")
	return line
}
