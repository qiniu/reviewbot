package stylecheck

import (
	"fmt"
	"github.com/qiniu/x/log"
	"regexp"
	"strconv"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

// refer to https://checkstyle.sourceforge.io/
const linterName = "stylecheck"

func init() {
	linters.RegisterPullRequestHandler(linterName, stylecheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})
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
			args = append(args, "-c", "/usr/local/rulesets/sun_checks.xml")
			a.LinterConfig.Args = args
			a.LinterConfig.Command = "java"
		}
	}

	return linters.GeneralHandler(log, a, func(l *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {

		output = []byte(TrimReport(string(output)))
		fmt.Println(string(output))
		return linters.Parse(log, output, stylecheckParser)
	})
}

func stylecheckParser(line string) (*linters.LinterOutput, error) {
	lineResult, err := StyleReportLineParser(line)
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

func StyleReportLineParser(line string) (*linters.LinterOutput, error) {
	log.Debugf("parse line: %s", line)

	patternColum := `^(.*):(\d+):(\d+): (.*)$`
	regexColum, errColum := regexp.Compile(patternColum)
	if errColum != nil {
		log.Errorf("compile regex failed: %v", errColum)
		return nil, errColum
	}
	pattern := `^(.*):(\d+): (.*)$`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		log.Errorf("compile regex failed: %v", err)
		return nil, err
	}
	matches := regexColum.FindStringSubmatch(line)
	if matches == nil {
		matches = regex.FindStringSubmatch(line)
	}
	if len(matches) < 4 {
		return nil, fmt.Errorf("unexpected format, original: %s", line)
	}

	lineNumber, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	var column int64
	message := matches[3]
	if len(matches) > 4 {
		columnNumber, err := strconv.ParseInt(matches[3], 10, 64)
		if err == nil {
			column = columnNumber
		}
		message = matches[4]

	}
	return &linters.LinterOutput{
		File:    matches[1],
		Line:    int(lineNumber),
		Column:  int(column),
		Message: message,
	}, nil
}
func TrimReport(line string) string {
	re := regexp.MustCompile("(?m)^.*检查.*$[\r\n]")
	reEnd := regexp.MustCompile("(?m)^.*错误结束.*$[\r\n]")
	line = re.ReplaceAllString(line, "")
	line = reEnd.ReplaceAllString(line, "")
	return line
}
