package luacheck

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

var lintName = "luacheck"

func init() {
	linters.RegisterCodeReviewHandler(lintName, luaCheckHandler)
}

func luaCheckHandler(log *xlog.Logger, linterConfig config.Linter, _ linters.Agent, _ github.PullRequestEvent) (map[string][]linters.LinterOutput, error) {
	executor, err := NewLuaCheckExecutor(linterConfig.WorkDir)
	if err != nil {
		log.Errorf("init luacheck executor failed: %v", err)
		return nil, err
	}

	if linters.IsEmpty(linterConfig.Args...) {
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

// luacheck is an executor that knows how to execute luacheck commands.
type luacheck struct {
	// dir is the location of this repo.
	dir string
	// luacheck is the path to the luacheck binary.
	luacheck string
}

func NewLuaCheckExecutor(dir string) (linters.Linter, error) {
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
	return linters.FormatLinterOutput(log, output, formatLuaCheckLine)
}

func formatLuaCheckLine(line string) (*linters.LinterOutput, error) {

	lineResult, err := linters.FormatLinterLine(line)
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
