/*
 Copyright 2024 Qiniu Cloud (qiniu.com).

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package staticcheck

import (
	"context"
	"os/exec"

	gh "github.com/qiniu/reviewbot/internal/github"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

var lintName = "staticcheck"

func init() {
	linters.RegisterPullRequestHandler(lintName, staticcheckHandler)
}

func staticcheckHandler(log *xlog.Logger, a linters.Agent) error {
	var (
		org     = a.PullRequestEvent.GetRepo().GetOwner().GetLogin()
		repo    = a.PullRequestEvent.GetRepo().GetName()
		num     = a.PullRequestEvent.GetNumber()
		orgRepo = org + "/" + repo
	)

	executor, err := NewStaticcheckExecutor(a.LinterConfig.WorkDir)
	if err != nil {
		log.Errorf("init staticcheck executor failed: %v", err)
		return err
	}

	if linters.IsEmpty(a.LinterConfig.Args...) {
		// turn off compile errors by default
		a.LinterConfig.Args = append([]string{}, "-debug.no-compile-errors=true", "./...")
	}

	output, err := executor.Run(log, a.LinterConfig.Args...)
	if err != nil {
		log.Errorf("staticcheck run failed: %v", err)
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
	log.Infof("staticcheck executor init")
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
			log.Printf("final command:  %v \n", c)
			return c.Output()
		},
	}, nil
}

func (e *Staticcheck) Run(log *xlog.Logger, args ...string) ([]byte, error) {
	b, err := e.execute(e.dir, e.staticcheck, args...)
	if err != nil {
		log.Errorf("staticcheck run with status: %v, mark and continue", err)
	} else {
		log.Infof("staticcheck succeeded")
	}

	return b, nil
}

// formatStaticcheckOutput formats the output of staticcheck
// example：
// gslb-api/utils.go:149:6: func dealWithRecordVipsId is unused (U1000)
// gslb-api/utils.go:162:2: this value of err is never used (SA4006)
// domain/repo/image.go:70:7: receiver name should be a reflection of its identity; don't use generic names such as "this" or "self" (ST1006)
//
//	output:  map[file][]linters.LinterOutput
func (e *Staticcheck) Parse(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	return linters.FormatLinterOutput(log, output, linters.FormatLinterLine)
}
