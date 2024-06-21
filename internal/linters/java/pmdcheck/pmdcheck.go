package pmdcheck

import (
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
	if (len(javaFiles) <= 0) || !linters.IsExist(rulePath) {
		return nil
	}
	if linters.IsEmpty(a.LinterConfig.Args...) {
		args := append([]string{}, "check")
		args = append(args, "-f", "emacs")
		args = append(args, javaFiles...)
		args = append(args, "-R", rulePath)
		a.LinterConfig.Args = args
	}
	if a.LinterConfig.Command == "" || a.LinterConfig.Command == linterName {
		a.LinterConfig.Command = "pmd"
	}
	if a.LinterConfig.LinterName == "" {
		a.LinterConfig.LinterName = linterName
	}

	return linters.GeneralHandler(log, a, func(l *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
		return linters.Parse(log, output, pmdcheckParser)
	})
}

func pmdcheckParser(line string) (*linters.LinterOutput, error) {
	if strings.Contains(line, "[WARN]") {
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
