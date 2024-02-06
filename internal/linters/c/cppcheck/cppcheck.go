package cppcheck

import (
	"bytes"
	"os/exec"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
	"github.com/reviewbot/config"
	"github.com/reviewbot/internal/linters"
)

var lintName = "cppcheck"

func init() {
	linters.RegisterCodeReviewHandler(lintName, cppcheckHandler)
}

func cppcheckHandler(log *xlog.Logger, linterConfig config.Linter, agent linters.Agent, event github.PullRequestEvent) (map[string][]linters.LinterOutput, error) {
	executor, err := NewCppcheckExecutor(linterConfig.WorkDir)
	if err != nil {
		log.Errorf("init cppcheck executor failed: %v", err)
		return nil, err
	}

	if linters.IsEmpty(linterConfig.Args...) {
		linterConfig.Args = append([]string{}, "--quiet", "--template='{file}:{line}:{column}: {message}'", "./")
	}

	output, err := executor.Run(log, linterConfig.Args...)
	if err != nil {
		log.Errorf("cppcheck run failed: %v", err)
		return nil, err
	}

	parsedOutput, err := executor.Parse(log, output)
	if err != nil {
		log.Errorf("cppcheck parse output failed: %v", err)
		return nil, err
	}

	return parsedOutput, nil
}

// Cppcheck is an executor that knows how to execute Cppcheck commands.
type Cppcheck struct {
	// dir is the location of this repo.
	dir string
	// cppcheck is the path to the cppcheck binary.
	cppcheck string
}

// NewCppcheckExecutor returns a new executor that knows how to execute cppcheck commands
// TODO: with config
func NewCppcheckExecutor(dir string) (linters.Linter, error) {
	log.Infof("cppcheck executor init")
	g, err := exec.LookPath("cppcheck")
	if err != nil {
		return nil, err
	}
	return &Cppcheck{
		dir:      dir,
		cppcheck: g,
	}, nil
}

func (e *Cppcheck) Run(log *xlog.Logger, args ...string) ([]byte, error) {
	c := exec.Command(e.cppcheck, args...)
	c.Dir = e.dir
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	if err != nil {
		log.Errorf("cppcheck run with status: %v, mark and continue", err)
	} else {
		log.Infof("cppcheck succeeded")
	}
	return out.Bytes(), nil
}

func (e *Cppcheck) Parse(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	return linters.FormatLinterOutput(log, output, linters.FormatLinterLine)
}
