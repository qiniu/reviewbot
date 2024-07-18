package golangcilint

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/xlog"
)

// refer to https://golangci-lint.run/
var lintName = "golangci-lint"

func init() {
	linters.RegisterPullRequestHandler(lintName, golangciLintHandler)
	linters.RegisterLinterLanguages(lintName, []string{".go"})
}

func golangciLintHandler(log *xlog.Logger, a linters.Agent) error {
	var gomodPaths []string
	if len(a.LinterConfig.Command) == 0 || (len(a.LinterConfig.Command) == 1 && a.LinterConfig.Command[0] == lintName) {
		// Default mode, automatically find the go.mod path in current repo
		a, gomodPaths = findAllFilesGoModsPath(a)
		// Default mode, automatically apply parameters.
		a = argsApply(log, a)
	} else if a.LinterConfig.ConfigPath != "" {
		// Custom mode, only apply golangci-lint configuration if necessary.
		path := configApply(log, a)
		log.Infof("golangci-lint config prepared: %v", path)
	}

	log.Infof("golangci-lint run config: %v", a.LinterConfig)

	if len(gomodPaths) > 0 {
		var outputs []byte
		execrun := func(a linters.Agent) ([]byte, error) {
			for _, gomodPath := range gomodPaths {
				a.LinterConfig.WorkDir = path.Dir(gomodPath)
				execGoModTidy(a.LinterConfig.WorkDir)
				output, err := linters.ExecRun(a)
				if err != nil {
					log.Warnf("golangci-lint run with exit code: %v, mark and continue", err)
				}
				outputs = append(outputs, output...)
			}
			return outputs, nil
		}

		return linters.GeneralHandler(log, a, execrun, parser)

	}

	return linters.GeneralHandler(log, a, linters.ExecRun, parser)
}

func parser(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
	var trainer = func(o linters.LinterOutput) (*linters.LinterOutput, []string) {
		// Perhaps it may not be precise enough？
		// refer: https://golangci-lint.run/usage/linters/
		if strings.Contains(o.Message, "(typecheck)") {
			unexpected := fmt.Sprintf("%s:%d:%d: %s", o.File, o.Line, o.Column, o.Message)
			return nil, []string{strings.TrimSpace(unexpected)}
		}

		return &o, []string{}
	}

	var unexpected = make([]string, 0)
	rawResults, rawUnexpected := linters.ParseV2(log, output, trainer)
	for _, ex := range rawUnexpected {
		// skip the warning level log
		// example: level=warning msg="[linters_context] copyloopvar: this linter is disabled because the Go version (1.18) of your project is lower than Go 1.22"
		// the warning level log is not a real lint error, so we need to skip it
		if strings.Contains(ex, "level=warning") {
			log.Warnf("skip golangci-lint warning: %s", ex)
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

	var newArgs = []string{"run"}

	var (
		timeoutFlag   bool
		parallelFlag  bool
		outFormatFlag bool
		printFlag     bool
		configFlag    bool
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
		}

		newArgs = append(newArgs, arg)
	}

	if !timeoutFlag {
		newArgs = append(newArgs, "--timeout=5m0s")
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
	var golangciConfigFiles = []string{".golangci.yml", ".golangci.yaml", ".golangci.json", ".golangci.toml"}

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

		var targetFile = filepath.Join(currentDir, filepath.Base(path))
		if err := os.WriteFile(targetFile, data, 0o600); err != nil {
			log.Warnf("failed to write config file: %v ,err: %v", targetFile, err)
			return ""
		}

		log.Infof("copy config file from %v to %v", path, targetFile)
		return targetFile
	}

	return path
}

// findAllFilesGoModsPath is used to determine and find out if there is go.mod file in the current project,
// if there is, it will store the gomod path in the array,
// if not, it will set the “GO111MODULE=off” environment variable.
func findAllFilesGoModsPath(a linters.Agent) (linters.Agent, []string) {
	var gomodPaths []string
	var fileDirPaths []string
	var pathmap = make(map[string]string)
	// When WorkDir and RepoDir are the same, then find the path where go.mod is located.
	if a.LinterConfig.WorkDir == a.RepoDir {
		for _, file := range a.PullRequestChangedFiles {
			if strings.HasSuffix(*file.Filename, ".go") {
				fileDirPaths = append(fileDirPaths, *file.Filename)
			}
		}
		for _, fileDirPath := range fileDirPaths {
			pathParts := strings.Split(fileDirPath, string(filepath.Separator))
			gomodpath := findGoMod(a, pathParts)
			if _, ok := pathmap[gomodpath]; !ok && gomodpath != "" {
				pathmap[gomodpath] = gomodpath
				gomodPaths = append(gomodPaths, gomodpath)
			}

		}

	}
	// When the go.mod file is not found, set GO111MODULE=off, so that golangci does not run through gomod.
	if len(gomodPaths) == 0 {
		a.LinterConfig.Env = append(a.LinterConfig.Env, "GO111MODULE=off")
		return a, []string{}
	}

	return a, gomodPaths
}

func findGoMod(a linters.Agent, pathParts []string) string {
	if path, exist := lintersutil.FileExists(a.RepoDir + "/" + "go.mod"); exist {
		return path
	}

	searchPath := a.RepoDir
	for k, v := range pathParts {
		if k == len(pathParts)-1 {
			break
		}
		searchPath = filepath.Join(searchPath, v)
		if path, exist := lintersutil.FileExists(searchPath + "/" + "go.mod"); exist {
			return path
		}
	}

	return ""
}

// Execute go mod tidy when go.mod exists.
func execGoModTidy(workdir string) {
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
