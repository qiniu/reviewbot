package luacheck

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	gh "github.com/qiniu/reviewbot/internal/github"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

var lintName = "luacheck"

func init() {
	linters.RegisterPullRequestHandler(lintName, luaCheckHandler)
}

func luaCheckHandler(log *xlog.Logger, a linters.Agent) error {
	var (
		org     = a.PullRequestEvent.GetRepo().GetOwner().GetLogin()
		repo    = a.PullRequestEvent.GetRepo().GetName()
		num     = a.PullRequestEvent.GetNumber()
		orgRepo = org + "/" + repo
	)
	
	executor, err := NewLuaCheckExecutor(a.LinterConfig.WorkDir)
	if err != nil {
		log.Errorf("init luacheck executor failed: %v", err)
		return err
	}

	if linters.IsEmpty(a.LinterConfig.Args...) {
		a.LinterConfig.Args = append([]string{}, ".")
	}

	output, err := executor.Run(log, a.LinterConfig.Args...)
	if err != nil {
		log.Errorf("luacheck run failed: %v", err)
		return err
	}

	lintResults, err := executor.Parse(log, output)
	if err != nil {
		log.Errorf("staticcheck parse output failed: %v", err)
		return err
	}

	if len(lintResults) == 0 {
		return nil
	}

	log.Infof("[%s] found total %d files with lint errors on repo %v", lintName, len(lintResults), orgRepo)
	comments, err := gh.BuildPullRequestCommentBody(lintName, lintResults, a.PullRequestChangedFiles)
	if err != nil {
		log.Errorf("failed to build pull request comment body: %v", err)
		return err
	}

	if len(comments) == 0 {
		// no related comments to post, continue to run other linters
		return nil
	}

	log.Infof("[%s] found valid %d comments related to this PR %d (%s) \n", lintName, len(comments), num, orgRepo)
	if err := gh.PostPullReviewCommentsWithRetry(context.Background(), a.GithubClient, org, repo, num, comments); err != nil {
		log.Errorf("failed to post comments: %v", err)
		return err
	}
	return nil

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
