package staticcheck

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/cr-bot/config"
	"github.com/cr-bot/linters"
	"github.com/qiniu/x/log"
)

func init() {
	linters.RegisterLinter("staticcheck", staticcheckHandler)
}

func staticcheckHandler(linterConfig config.Linter) (map[string][]linters.LinterOutput, error) {
	executor, err := NewStaticcheckExecutor(linterConfig.WorkDir)
	if err != nil {
		return nil, err
	}

	output, err := executor.Run(linterConfig.Args...)
	if err != nil {
		return nil, err
	}

	parsedOutput, err := executor.Parse(output)
	if err != nil {
		return nil, err
	}

	return parsedOutput, nil
}

// Staticcheck is an executor that knows how to execute Staticcheck commands.
type Staticcheck struct {
	// dir is the location of this repo.
	dir string
	// staticcheck is the path to the staticcheck binary.
	staticcheck string
	// execute executes a command
	execute func(dir, command string, args ...string) ([]byte, error)
}

// NewStaticcheckExecutor returns a new executor that knows how to execute staticcheck commands
// TODO: with config
func NewStaticcheckExecutor(dir string) (linters.Linter, error) {
	g, err := exec.LookPath("staticcheck")
	if err != nil {
		return nil, err
	}
	return &Staticcheck{
		dir:         dir,
		staticcheck: g,
		execute: func(dir, command string, args ...string) ([]byte, error) {
			c := exec.Command(command, args...)
			c.Dir = dir
			return c.Output()
		},
	}, nil
}

func (e *Staticcheck) Run(args ...string) ([]byte, error) {
	b, err := e.execute(e.dir, e.staticcheck, args...)
	if err != nil {
		log.Errorf("staticcheck run with status: %v, mark and continue", err)
	} else {
		log.Infof("staticcheck succeeded")
	}

	return b, nil
}

func (e *Staticcheck) Parse(output []byte) (map[string][]linters.LinterOutput, error) {
	return formatStaticcheckOutput(output)
}

// formatStaticcheckOutput formats the output of staticcheck
// example：
// gslb-api/utils.go:149:6: func dealWithRecordVipsId is unused (U1000)
// gslb-api/utils.go:162:2: this value of err is never used (SA4006)
// domain/repo/image.go:70:7: receiver name should be a reflection of its identity; don't use generic names such as "this" or "self" (ST1006)
//
//	output:  map[file][]linters.LinterOutput
func formatStaticcheckOutput(output []byte) (map[string][]linters.LinterOutput, error) {
	lines := strings.Split(string(output), "\n")

	var result = make(map[string][]linters.LinterOutput)
	for _, line := range lines {
		if line == "" {
			continue
		}
		output, err := formatStaticcheckLine(line)
		if err != nil {
			log.Warnf("unexpected staticcheck output: %v", line)
			// 不直接退出
			continue
		}

		if output == nil {
			continue
		}

		if outs, ok := result[output.File]; !ok {
			result[output.File] = []linters.LinterOutput{*output}
		} else {
			outs = append(outs, *output)
			result[output.File] = outs
		}
	}

	return result, nil
}

func formatStaticcheckLine(line string) (*linters.LinterOutput, error) {
	pattern := `^(.*):(\d+):(\d+): (.*) \((.*)\)$`
	regex := regexp.MustCompile(pattern)
	matches := regex.FindStringSubmatch(line)
	if len(matches) != 6 {
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
		Label:   matches[5],
	}, nil
}
