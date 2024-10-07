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

package golangcilint

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

// refer to https://golangci-lint.run/
var lintName = "golangci-lint"

func init() {
	linters.RegisterPullRequestHandler(lintName, golangciLintHandler)
	linters.RegisterLinterLanguages(lintName, []string{".go", ".mod", ".sum"})
}

func golangciLintHandler(ctx context.Context, a linters.Agent) error {
	log := lintersutil.FromContext(ctx)
	var goModDirs []string
	if len(a.LinterConfig.Command) == 0 || (len(a.LinterConfig.Command) == 1 && a.LinterConfig.Command[0] == lintName) {
		// Default mode, automatically find the go.mod path in current repo
		goModDirs = findGoModDirs(a)
		log.Infof("find go.mod in dirs: %v", goModDirs)
		// Default mode, automatically apply parameters.
		a = argsApply(log, a)
	} else if a.LinterConfig.ConfigPath != "" {
		// Custom mode, only apply golangci-lint configuration if necessary.
		path := configApply(log, a)
		log.Infof("golangci-lint config prepared: %v", path)
	}

	log.Infof("golangci-lint run config: %v", a.LinterConfig)

	// When the go.mod file is not found, set GO111MODULE=off, so that golangci does not run through gomod.
	if len(goModDirs) == 0 {
		a.LinterConfig.Env = append(a.LinterConfig.Env, "GO111MODULE=off")
		return linters.GeneralHandler(ctx, log, a, linters.ExecRun, parser)
	}

	a.LinterConfig.Modifier = newGoModTidyBuilder(a.LinterConfig.Modifier, goModDirs)
	return linters.GeneralHandler(ctx, log, a, linters.ExecRun, parser)
}

type goModTidyModifier struct {
	next      config.Modifier
	goModDirs []string
}

func newGoModTidyBuilder(next config.Modifier, goModDirs []string) config.Modifier {
	return &goModTidyModifier{
		next:      next,
		goModDirs: goModDirs,
	}
}

func (b *goModTidyModifier) Modify(cfg *config.Linter) (*config.Linter, error) {
	base, err := b.next.Modify(cfg)
	if err != nil {
		return nil, err
	}

	newCfg := base
	args := []string{}
	for _, dir := range b.goModDirs {
		args = append(args, fmt.Sprintf("cd %s > /dev/null && go mod tidy && cd - > /dev/null \n", dir))
	}

	newCfg.Args = append(args, base.Args...)
	return newCfg, nil
}

func parser(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
	log.Infof("golangci-lint output: %s", output)
	trainer := func(o linters.LinterOutput) (*linters.LinterOutput, []string) {
		// Perhaps it may not be precise enough？
		// refer: https://golangci-lint.run/usage/linters/
		if strings.Contains(o.Message, "(typecheck)") {
			unexpected := fmt.Sprintf("%s:%d:%d: %s", o.File, o.Line, o.Column, o.Message)
			return nil, []string{strings.TrimSpace(unexpected)}
		}

		return &o, []string{}
	}

	unexpected := make([]string, 0)
	rawResults, rawUnexpected := linters.ParseV2(log, output, trainer)
	for _, ex := range rawUnexpected {
		// skip the warning level log
		// example: level=warning msg="[linters_context] copyloopvar: this linter is disabled because the Go version (1.18) of your project is lower than Go 1.22"
		// the warning level log is not a real lint error, so we need to skip it
		if strings.Contains(ex, "level=warning") {
			continue
		}

		// skip the go download log
		if strings.Contains(ex, "go: downloading") || strings.Contains(ex, "go: finding") {
			continue
		}

		// skip the docker artifact log flag
		if strings.Contains(ex, "---artifacts-") {
			continue
		}

		unexpected = append(unexpected, strings.TrimSpace(ex))
	}

	return rawResults, unexpected
}

// argsApply is used to set the default parameters for golangci-lint
// see: ./docs/website/docs/component/go/golangci-lint
func argsApply(log *xlog.Logger, a linters.Agent) linters.Agent {
	config := a.LinterConfig
	if len(config.Command) == 0 || len(config.Command) > 1 || config.Command[0] != lintName {
		return a
	}

	legacyArgs := config.Args

	switch {
	case len(legacyArgs) == 0:
		legacyArgs = []string{}
	case len(legacyArgs) > 0 && legacyArgs[0] != "run":
		return a
	default:
		legacyArgs = legacyArgs[1:]
	}

	newArgs := []string{"run"}

	var (
		timeoutFlag     bool
		parallelFlag    bool
		outFormatFlag   bool
		printFlag       bool
		configFlag      bool
		concurrencyFlag bool
	)

	for _, arg := range legacyArgs {

		switch {
		case strings.HasPrefix(arg, "--timeout"):
			timeoutFlag = true
		case strings.HasPrefix(arg, "--allow-parallel-runners"):
			parallelFlag = true
		case strings.HasPrefix(arg, "--out-format"):
			outFormatFlag = true
		case strings.HasPrefix(arg, "--print-issued-lines"):
			printFlag = true
		case strings.HasPrefix(arg, "--config"):
			configFlag = true
		case strings.HasPrefix(arg, "--concurrency"):
			concurrencyFlag = true
		}

		newArgs = append(newArgs, arg)
	}

	if !timeoutFlag {
		newArgs = append(newArgs, "--timeout=15m0s")
	}
	if !parallelFlag {
		newArgs = append(newArgs, "--allow-parallel-runners=true")
	}
	if !outFormatFlag {
		newArgs = append(newArgs, "--out-format=line-number")
	}
	if !printFlag {
		newArgs = append(newArgs, "--print-issued-lines=false")
	}
	if !concurrencyFlag {
		newArgs = append(newArgs, "--concurrency=8")
	}
	if !configFlag && config.ConfigPath != "" {
		config.ConfigPath = configApply(log, a)
		newArgs = append(newArgs, "--config", config.ConfigPath)
	}

	config.Args = newArgs
	a.LinterConfig = config

	return a
}

// configApply is used to get the config file path based on rules as below:
// 1. if the config file exists in current directory, return its absolute path.
// 2. if the config file exists in the workDir directory, return its absolute path.
// 3. if the config file exists in the repo linter ConfigPath.
func configApply(log *xlog.Logger, a linters.Agent) string {
	// refer to https://golangci-lint.run/usage/configuration/
	// the default config file name is .golangci.yml, .golangci.yaml, .golangci.json, .golangci.toml
	golangciConfigFiles := []string{".golangci.yml", ".golangci.yaml", ".golangci.json", ".golangci.toml"}

	// if the config file exists in the current directory, return its absolute path
	for _, file := range golangciConfigFiles {
		if path, exist := lintersutil.FileExists(file); exist {
			return path
		}
	}

	// if the config file exists in the workDir directory, return its absolute path
	if a.LinterConfig.WorkDir != "" {
		for _, file := range golangciConfigFiles {
			if path, exist := lintersutil.FileExists(a.LinterConfig.WorkDir + "/" + file); exist {
				return path
			}
		}
	}

	if a.LinterConfig.ConfigPath == "" {
		return ""
	}

	path, exist := lintersutil.FileExists(a.LinterConfig.ConfigPath)
	if !exist {
		// if the config file is not found, these probably not configured.
		// here we still return the original config path which may let user know the config is wrong.
		return a.LinterConfig.ConfigPath
	}

	// WorkDir is the working directory of the linter, it is a temporary directory
	currentDir := a.LinterConfig.WorkDir

	// if the config file is outside the repo, copy it to the repo
	if !strings.Contains(path, currentDir) {
		// copy the config file to the repo
		data, err := os.ReadFile(path)
		if err != nil {
			log.Warnf("failed to read config file: %v, err: %v", path, err)
			return ""
		}

		targetFile := filepath.Join(currentDir, filepath.Base(path))
		if err := os.WriteFile(targetFile, data, 0o600); err != nil {
			log.Warnf("failed to write config file: %v ,err: %v", targetFile, err)
			return ""
		}

		log.Infof("copy config file from %v to %v", path, targetFile)
		return targetFile
	}

	return path
}

// findGoModDirs aims to find the dirs where go.mod files exist in the current repo based on the changed files.
func findGoModDirs(a linters.Agent) []string {
	// it means WorkDir is specified via the config file probably, so we don't need to find go.mod
	if a.LinterConfig.WorkDir != a.RepoDir {
		log.Infof("WorkDir does not match the repo dir, so we don't need to find go.mod. WorkDir: %v, RepoDir: %v", a.LinterConfig.WorkDir, a.RepoDir)
		return []string{}
	}

	var goModDirs []string
	dirs := extractDirs(a.PullRequestChangedFiles)
	for _, dir := range dirs {
		goModFile := filepath.Join(a.RepoDir, dir, "go.mod")
		if _, exist := lintersutil.FileExists(goModFile); exist {
			goModDirs = append(goModDirs, filepath.Join(a.RepoDir, dir))
		}
	}
	return goModDirs
}

func extractDirs(commitFiles []*github.CommitFile) []string {
	directorySet := make(map[string]bool)

	for _, file := range commitFiles {
		if filepath.Ext(file.GetFilename()) != ".go" && filepath.Ext(file.GetFilename()) != ".mod" && filepath.Ext(file.GetFilename()) != ".sum" {
			continue
		}
		dir := filepath.Dir(file.GetFilename())

		// Split the directory path into individual directories
		dirs := strings.Split(dir, string(filepath.Separator))
		prefix := "."
		// root directory
		directorySet[prefix] = true
		for _, d := range dirs {
			prefix = filepath.Join(prefix, d)
			// current directory
			directorySet[prefix] = true
		}
	}

	directories := make([]string, 0, len(directorySet))
	for dir := range directorySet {
		directories = append(directories, dir)
	}

	return directories
}

// Execute go mod tidy when go.mod exists.
func execGoModTidy(log *xlog.Logger, workdir string) {
	log.Infof("go mod tidy workdir: %v", workdir)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = workdir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Warnf("Error running go mod tidy:%v", err)
	} else {
		log.Info("running go mod tidy successfully")
	}
	if stderr.Len() > 0 {
		log.Warnf("running go mod tidy something wrong :%s", stderr.String())
	}
}
