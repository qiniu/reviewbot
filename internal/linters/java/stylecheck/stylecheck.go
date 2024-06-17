package stylecheck

import (
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/x/log"
	"regexp"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

const linterName = "stylecheck"

var rulePath string

func init() {
	linters.RegisterPullRequestHandler(linterName, stylecheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})
	var c config.GlobalConfig
	rulePath = c.JavaStyleCheckRuleConfig
}

func stylecheckHandler(log *xlog.Logger, a linters.Agent) error {
	var javaFiles []string
	for _, arg := range a.PullRequestChangedFiles {
		if strings.HasSuffix(arg.GetFilename(), ".java") {
			javaFiles = append(javaFiles, a.LinterConfig.WorkDir+"/"+arg.GetFilename())
		}
	}

	if len(javaFiles) > 0 {
		if linters.IsEmpty(a.LinterConfig.Args...) {
			//args := append([]string{}, "-jar", "/usr/local/checkstyle-10.17.0-all.jar")
			args := append([]string{}, "-jar", "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/checkstyle-10.17.0-all.jar")
			args = append(args, javaFiles...)
			args = append(args, "-c", a.LinterConfig.ConfigPath)
			a.LinterConfig.Args = args
			a.LinterConfig.Command = "java"
			a.LinterConfig.LinterName = "stylecheck"

		}
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
