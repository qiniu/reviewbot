package govet

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
	"github.com/reviewbot/config"
	"github.com/reviewbot/internal/linters"
)

var lintName = "govet"

func init() {
	linters.RegisterCodeReviewHandler(lintName, goVetHandler)
}

func goVetHandler(log *xlog.Logger, linterConfig config.Linter, _ linters.Agent, _ github.PullRequestEvent) (map[string][]linters.LinterOutput, error) {
	executor, err := NewGoVetExecutor(linterConfig.WorkDir)
	if err != nil {
		log.Errorf("init govet executor failed: %v", err)
		return nil, err
	}

	if isEmpty(linterConfig.Args...) {
		linterConfig.Args = append([]string{}, "vet", "./...")
	}

	output, err := executor.Run(log, linterConfig.Args...)
	if err != nil {
		log.Errorf("govet run failed: %v", err)
		return nil, err
	}

	parsedOutput, err := executor.Parse(log, output)
	if err != nil {
		log.Errorf("govet parse output failed: %v", err)
		return nil, err
	}
	return parsedOutput, nil
}

func isEmpty(args ...string) bool {
	for _, arg := range args {
		if arg != "" {
			return false
		}
	}

	return true
}

// govet is an executor that knows how to execute govet commands.
type govet struct {
	// dir is the location of this repo.
	dir string
	// govet is the path to the govet binary.
	govet string
}

func NewGoVetExecutor(dir string) (linters.Linter, error) {
	g, err := exec.LookPath("go")
	if err != nil {
		return nil, err
	}
	return &govet{
		dir:   dir,
		govet: g,
	}, nil
}

func (e *govet) Run(log *xlog.Logger, args ...string) ([]byte, error) {
	c := exec.Command(e.govet, args...)
	c.Dir = e.dir
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	if err != nil {
		log.Errorf("go vet run with status: %v, mark and continue", err)
	} else {
		log.Infof("go vet succeeded")
	}

	return out.Bytes(), nil
}

func (e *govet) Parse(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	return formatGoVetOutput(log, output)
}

func formatGoVetOutput(log *xlog.Logger, out []byte) (map[string][]linters.LinterOutput, error) {
	lines := strings.Split(string(out), "\n")

	var result = make(map[string][]linters.LinterOutput)
	for _, line := range lines {
		if line == "" {
			continue
		}
		output, err := formatGoVetLine(line)

		if err != nil {
			log.Warnf("unexpected govet output: %v", line)
			// 不直接退出
			continue
		}

		if output == nil {
			continue
		}

		if outs, ok := result[output.File]; !ok {
			result[output.File] = []linters.LinterOutput{*output}
		} else {
			// remove duplicate
			var existed bool
			for _, v := range outs {
				if v.File == output.File && v.Line == output.Line &&
					v.Column == output.Column && v.Message == output.Message {
					existed = true
					break
				}
			}

			if !existed {
				result[output.File] = append(result[output.File], *output)
			}
		}
	}

	return result, nil
}

func formatGoVetLine(line string) (*linters.LinterOutput, error) {
	pattern := `^(.*):(\d+):(\d+): (.*)$`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		log.Errorf("compile regex failed: %v", err)
		return nil, err
	}
	matches := regex.FindStringSubmatch(line)
	if len(matches) != 5 {
		return nil, fmt.Errorf("unexpected format, original: %s", line)
	}

	lineNumber, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return nil, err
	}

	columnNumber, err := strconv.ParseInt(matches[3], 10, 64)
	if err != nil {
		return nil, err
	}

	return &linters.LinterOutput{
		File:    matches[1],
		Line:    int(lineNumber),
		Column:  int(columnNumber),
		Message: matches[4],
	}, nil
}
