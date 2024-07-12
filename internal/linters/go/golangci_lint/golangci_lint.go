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
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

// refer to https://golangci-lint.run/
var lintName = "golangci-lint"

func init() {
	linters.RegisterPullRequestHandler(lintName, golangciLintHandler)
	linters.RegisterLinterLanguages(lintName, []string{".go"})
}

func golangciLintHandler(log *xlog.Logger, a linters.Agent) error {
	if len(a.LinterConfig.Command) == 0 || (len(a.LinterConfig.Command) == 1 && a.LinterConfig.Command[0] == lintName) {
		// Default mode, automatically apply golangci workdir
		a = workDirApply(a)
		// Default mode, automatically apply parameters.
		a = argsApply(a)
	} else if a.LinterConfig.ConfigPath != "" {
		// Custom mode, only apply golangci-lint configuration if necessary.
		path := configApply(a)
		log.Infof("golangci-lint config prepared: %v", path)
	}

	log.Infof("golangci-lint run config: %v", a.LinterConfig)
	return linters.GeneralHandler(log, a, parser)
}

func parser(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
	var lineParser = func(line string) (*linters.LinterOutput, error) {
		if strings.HasSuffix(line, "(typecheck)") {
			// refer: https://github.com/qiniu/reviewbot/issues/82#issuecomment-2002340788
			return nil, fmt.Errorf("skip golangci-lint typecheck error: %s", line)
		}

		// skip the warning level log
		// example: level=warning msg="[linters_context] copyloopvar: this linter is disabled because the Go version (1.18) of your project is lower than Go 1.22"
		// the warning level log is not a real lint error, so we need to skip it
		if strings.Contains(line, "level=warning") {
			log.Warnf("skip golangci-lint warning: %s", line)
			return nil, nil
		}

		return linters.GeneralLineParser(line)
	}

	return linters.Parse(log, output, lineParser)
}

// argsApply is used to set the default parameters for golangci-lint
// see: ./docs/website/docs/component/go/golangci-lint
func argsApply(a linters.Agent) linters.Agent {
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
		config.ConfigPath = configApply(a)
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
func configApply(a linters.Agent) string {
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

// workDirApply is used to configure the default execution path of golangci-lint when the user does not customize it.
func workDirApply(a linters.Agent) linters.Agent {
	var gomodPath string
	var fileDirPath string

	// When WorkDir and RepoDir are the same, then find the path where go.mod is located.
	if a.LinterConfig.WorkDir == a.RepoDir {
		for _, file := range a.PullRequestChangedFiles {
			if strings.HasSuffix(*file.Filename, ".go") {
				fileDirPath = *file.Filename
				break
			}
		}
		pathParts := strings.Split(fileDirPath, string(filepath.Separator))
		gomodPath = findGoMod(a, pathParts)
	}
	// When the go.mod file is not found, set GO111MODULE=off, so that golangci does not run through gomod.
	if gomodPath == "" {
		a.LinterConfig.Env = append(a.LinterConfig.Env, "GO111MODULE=off")
		return a
	}

	a.LinterConfig.WorkDir = path.Dir(gomodPath)

	// Execute go mod tidy when go.mod exists.
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = a.LinterConfig.WorkDir
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

	return a
}

func findGoMod(a linters.Agent, pathParts []string) string {
	if path, exist := lintersutil.FileExists(a.RepoDir + "/" + "go.mod"); exist {
		return path
	}

	searchPath := a.RepoDir
	for k, v := range pathParts {
		if k == len(pathParts) {
			break
		}
		searchPath = filepath.Join(searchPath, v)
		if path, exist := lintersutil.FileExists(searchPath + "/" + "go.mod"); exist {
			return path
		}
	}

	return ""
}
