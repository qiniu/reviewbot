package gomodcheck

import (
	"bufio"
	"context"
	"os"
	"regexp"

	"path/filepath"
	"strings"

	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/linters"

	"github.com/qiniu/x/xlog"
)

var lintName = "gomodcheck"

func init() {
	linters.RegisterPullRequestHandler(lintName, goModCheckHandler)
	linters.RegisterLinterLanguages(lintName, []string{".go", ".mod"})
}

func goModCheckHandler(ctx context.Context, a linters.Agent) error {
	log := xlog.New(ctx.Value(config.EventGUIDKey).(string))
	parsedOutput, err := goModCheckOutput(log, a)
	if err != nil {
		log.Errorf("gomodchecks parse output failed: %v", err)
		return err
	}
	return linters.Report(log, a, parsedOutput)

}

func goModCheckOutput(log *xlog.Logger, a linters.Agent) (map[string][]linters.LinterOutput, error) {
	output := make(map[string][]linters.LinterOutput)
	for _, file := range a.PullRequestChangedFiles {
		if filepath.Ext(file.GetFilename()) != ".mod" {
			continue
		}

		replaceRegex := regexp.MustCompile(`^replace\s+([^\s]+)\s+=>\s+([^\s]+)`)
		goModPath := filepath.Join(a.RepoDir, file.GetFilename())
		file, err := os.Open(goModPath)
		if err != nil {
			log.Errorf("Error opening %s: %s", goModPath, err)
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNumber := 0
		filename := strings.TrimPrefix(goModPath, a.RepoDir+"/")
		msg := "It is not recommended to use `replace ../xxx` to specify dependency "

		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if matches := replaceRegex.FindStringSubmatch(line); len(matches) > 0 {
				replacementPath := matches[2]
				if strings.HasPrefix(replacementPath, "../") {
					output[filename] = append(output[filename], linters.LinterOutput{
						File:    filename,
						Line:    lineNumber,
						Column:  1,
						Message: msg,
					})
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Errorf("Error reading go.mod: %s", err)
			continue
		}

	}

	return output, nil
}
