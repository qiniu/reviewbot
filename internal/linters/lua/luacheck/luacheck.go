package luacheck

import (
	"bytes"
	"fmt"
	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
	"github.com/reviewbot/config"
	"github.com/reviewbot/internal/linters"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var lintName = "luacheck"

func init() {
	linters.RegisterCodeReviewHandler(lintName, luaCheckHandler)
}

func luaCheckHandler(log *xlog.Logger, linterConfig config.Linter, _ linters.Agent, _ github.PullRequestEvent) (map[string][]linters.LinterOutput, error) {
	executor, err := NewGoVetExecutor(linterConfig.WorkDir)
	if err != nil {
		log.Errorf("init luacheck executor failed: %v", err)
		return nil, err
	}

	if isEmpty(linterConfig.Args...) {
		linterConfig.Args = append([]string{}, ".")
	}

	output, err := executor.Run(log, linterConfig.Args...)
	if err != nil {
		log.Errorf("luacheck run failed: %v", err)
		return nil, err
	}

	parsedOutput, err := executor.Parse(log, output)
	if err != nil {
		log.Errorf("luacheck parse output failed: %v", err)
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

// luacheck is an executor that knows how to execute luacheck commands.
type luacheck struct {
	// dir is the location of this repo.
	dir string
	// luacheck is the path to the luacheck binary.
	luacheck string
}

func NewGoVetExecutor(dir string) (linters.Linter, error) {
	g, err := exec.LookPath("luacheck")
	if err != nil {
		return nil, err
	}
	return &luacheck{
		dir:      dir,
		luacheck: g,
	}, nil
}

func (e *luacheck) Run(log *xlog.Logger, args ...string) ([]byte, error) {
	c := exec.Command(e.luacheck, args...)
	c.Dir = e.dir
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	if err != nil {
		log.Errorf("luacheck run with status: %v, mark and continue", err)
	} else {
		log.Infof("luacheck succeeded")
	}

	return out.Bytes(), nil
}

func (e *luacheck) Parse(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	return formatLuaCheckOutput(log, output)
}

func formatLuaCheckOutput(log *xlog.Logger, out []byte) (map[string][]linters.LinterOutput, error) {
	lines := strings.Split(string(out), "\n")

	var result = make(map[string][]linters.LinterOutput)
	for _, line := range lines {
		//  " access/access.lua:1:14: accessing undefined variable [0m[1mngx[0m"
		if line == "" {
			continue
		}
		output, err := formatLuaCheckLine(line)

		if err != nil {
			log.Warnf("unexpected luacheck output: %v", line)
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

func formatLuaCheckLine(line string) (*linters.LinterOutput, error) {
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
		File:   strings.TrimLeft(matches[1], " "),
		Line:   int(lineNumber),
		Column: int(columnNumber),
		//Message: strings.ReplaceAll(strings.ReplaceAll(matches[4], "\x1b[0m[1m", ""), "\x1b[0m", ""),
		Message: strings.ReplaceAll(strings.ReplaceAll(matches[4], "\x1b[0m\x1b[1m", ""), "\x1b[0m", ""),
	}, nil
}
