package stylecheck

import (
	"embed"
	"github.com/qiniu/x/log"
	"os"
	"regexp"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

const linterName = "stylecheck"

//go:embed ruleset/*
var resources embed.FS
var rulePath, ruleDir string

func init() {
	linters.RegisterPullRequestHandler(linterName, stylecheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})
	rulePath, ruleDir = initCheckStyleRule()
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
		os.Remove(ruleDir)
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
func initCheckStyleRule() (string, string) {

	tempDir, temdirerr := os.CreateTemp("", "*sun_checks.xml")
	if temdirerr != nil {
		log.Errorf("pmd rule temp dir error: %v", temdirerr)
	}
	//rulePath := filepath.Join(tempDir.Name(), "bestpractices.xml")
	//newfile, fileerr := os.Create(rulePath)
	//if fileerr != nil {
	//	log.Errorf("pmd rule file create error: %v", fileerr)
	//}
	content, readerr := resources.ReadFile("ruleset/sun_checks.xml")
	if readerr != nil {
		log.Errorf("style rule resource read  error: %v", readerr)
	}
	_, err := tempDir.Write(content)
	if err != nil {
		log.Errorf("style rule resource write error: %v", err)
	}
	closeErr := tempDir.Close()
	if closeErr != nil {
		log.Errorf("style rule resource close error: %v", closeErr)
	}
	return tempDir.Name(), tempDir.Name()
}
