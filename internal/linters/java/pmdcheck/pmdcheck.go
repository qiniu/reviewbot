package pmdcheck

import (
	"embed"
	"github.com/qiniu/x/log"
	"os"
	"regexp"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

// refer to https://pmd.github.io/
const linterName = "pmdcheck"

var pmdRule, rulPath string

//go:embed ruleset/*
var resources embed.FS

func init() {
	linters.RegisterPullRequestHandler(linterName, pmdcheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})
	pmdRule, rulPath = initPmdRule()
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
			args = append(args, "-R", pmdRule)
			a.LinterConfig.Args = args
			a.LinterConfig.Command = "pmd"
		}
	}

	return linters.GeneralHandler(log, a, func(l *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
		output = []byte(TrimReport(string(output)))
		os.RemoveAll(rulPath)
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

func TrimReport(line string) string {
	re := regexp.MustCompile("(?m)^.*WARN.*$[\r\n]")
	line = re.ReplaceAllString(line, "")
	return line
}
func initPmdRule() (string, string) {
	tempDir, temdirerr := os.CreateTemp("", "*bestpractices.xml")
	if temdirerr != nil {
		log.Errorf("pmd rule temp dir error: %v", temdirerr)
	}
	//rulePath := filepath.Join(tempDir.Name(), "bestpractices.xml")
	//newfile, fileerr := os.Create(rulePath)
	//if fileerr != nil {
	//	log.Errorf("pmd rule file create error: %v", fileerr)
	//}
	content, readerr := resources.ReadFile("ruleset/bestpractices.xml")
	if readerr != nil {
		log.Errorf("pmd rule resource read  error: %v", readerr)
	}
	_, err := tempDir.Write(content)
	if err != nil {
		log.Errorf("pmd rule resource write  error: %v", err)
	}
	closeErr := tempDir.Close()
	if closeErr != nil {
		log.Errorf("pmd rule resource ckose  error: %v", closeErr)
	}
	return tempDir.Name(), tempDir.Name()
}
