package pmdcheck

import (
	"fmt"
	"github.com/qiniu/x/log"
	"regexp"
	"strconv"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

// refer to https://github.com/mpeterv/luacheck
const linterName = "pmdcheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, pmdcheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})
}

func pmdcheckHandler(log *xlog.Logger, a linters.Agent) error {
	//if linters.IsEmpty(a.LinterConfig.Args...) {
	//	a.LinterConfig.Args = append([]string{}, ".")
	//}
	var javaFiles []string
	for _, arg := range a.PullRequestChangedFiles {
		if strings.HasSuffix(arg.GetFilename(), ".java") {
			javaFiles = append(javaFiles, arg.GetFilename())
		}
	}

	if len(javaFiles) > 0 {
		//cmd := a.LinterConfig.Command
		// execute shellcheck with the following command
		// shellcheck -f gcc xxx.sh...
		if linters.IsEmpty(a.LinterConfig.Args...) {
			// use gcc format to make the output more readable
			args := append([]string{}, "check")
			args = append(args, "-f", "emacs")

			args = append(args, javaFiles...)
			args = append(args, "-R", "/usr/local/rulesets/bestpractices.xml")
			//args = append(args, "-r", "pdm.txt")
			a.LinterConfig.Args = args
			a.LinterConfig.Command = "pmd"
		}
	}

	return linters.GeneralHandler(log, a, func(l *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
		output = []byte(TrimReport(string(output)))
		return linters.Parse(log, output, pmdcheckParser)
	})
}

func pmdcheckParser(line string) (*linters.LinterOutput, error) {
	lineResult, err := PmdReportLineParser(line)
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

func PmdReportLineParser(line string) (*linters.LinterOutput, error) {
	log.Debugf("parse line: %s", line)
	pattern := `^(.*):(\d+): (.*)$`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		log.Errorf("compile regex failed: %v", err)
		return nil, err
	}
	matches := regex.FindStringSubmatch(line)
	if len(matches) != 4 {
		return nil, fmt.Errorf("unexpected format, original: %s", line)
	}

	lineNumber, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return nil, err
	}

	//columnNumber, err := strconv.ParseInt(matches[3], 10, 64)
	//columnNumber := 4
	if err != nil {
		return nil, err
	}

	return &linters.LinterOutput{
		File: matches[1],
		Line: int(lineNumber),
		//Column:  int(columnNumber),
		Message: matches[3],
	}, nil
}
func TrimReport(line string) string {
	//fmt.Println(line)
	re := regexp.MustCompile("(?m)^.*WARN.*$[\r\n]")
	//re1 := regexp.MustCompile("(?m)^.*WARN.*")
	line = re.ReplaceAllString(line, "")
	//fmt.Println(line)
	return line
}
