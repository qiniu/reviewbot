package shellcheck

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

var lintName = "shellcheck"

func init() {
	linters.RegisterPullRequestHandler(lintName, shellcheckHandler)
}

func shellcheckHandler(log *xlog.Logger, a linters.Agent) error {
	/*
	 * Shellcheck can not support check directories recursively, it is usually used in conjunction with the "find" command
	 * Info:
	 *  https://github.com/koalaman/shellcheck/issues/143
	 *  https://github.com/koalaman/shellcheck/wiki/Recursiveness
	 */
	if linters.IsEmpty(a.LinterConfig.Args...) {
		paths := ListShellFileFromPR(a)
		// paths := ListShellFileFromRepoPath(a.LinterConfig.WorkDir)
		a.LinterConfig.Args = append(a.LinterConfig.Args, paths...)
	}

	executor, err := NewShellcheckExecutor(a.LinterConfig.WorkDir)
	if err != nil {
		log.Errorf("init shellcheck executor failed: %v", err)
		return err
	}

	output, err := executor.Run(log, a.LinterConfig.Args...)
	if err != nil {
		log.Errorf("shellcheck run failed: %v", err)
		return err
	}
	parsedOutput, err := executor.Parse(log, output)
	if err != nil {
		log.Errorf("shellcheck parse output failed: %v", err)
		return err
	}

	return linters.Report(log, a, parsedOutput)
}

func ListShellFileFromRepoPath(dir string) []string {
	c := exec.Command("find", ".", "-type", "f", "-name", "*.sh")
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		log.Errorf("find the path of shell files failed, err: %v", err)
	}
	fps := strings.Split(string(out), "\n")
	for i := 0; i < len(fps); i++ {
		fps[i] = strings.TrimPrefix(fps[i], "./")
	}
	return fps
}

func ListShellFileFromPR(a linters.Agent) []string {
	var Files []string
	for _, v := range a.PullRequestChangedFiles {
		if strings.HasSuffix(*v.Filename, ".sh") {
			Files = append(Files, *v.Filename)
		}
	}
	return Files
}

type Shellcheck struct {
	dir        string
	shellcheck string
	execute    func(dir, command string, args ...string) ([]byte, error)
}

func NewShellcheckExecutor(dir string) (linters.Linter, error) {
	log.Infof("shellcheck executor init")
	g, err := exec.LookPath("shellcheck")
	if err != nil {
		return nil, err
	}
	return &Shellcheck{
		dir:        dir,
		shellcheck: g,
		execute: func(dir, command string, args ...string) ([]byte, error) {
			c := exec.Command(command, args...)
			c.Dir = dir
			log.Printf("final command:  %v \n", c)
			return c.Output()
		},
	}, nil
}

func (g *Shellcheck) Run(log *xlog.Logger, args ...string) ([]byte, error) {
	b, err := g.execute(g.dir, g.shellcheck, args...)
	if err != nil {
		log.Errorf("shellcheck run with status: %v, mark and continue", err)
	} else {
		log.Infof("shellcheck running succeeded")
	}
	return b, nil
}

func (g *Shellcheck) Parse(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	log.Infof("shellcheck output is being parsed")
	return formatShellcheckOutput(output)
}

// The wiki link of the shellcheck: https://github.com/koalaman/shellcheck/wiki
func formatShellcheckOutput(output []byte) (map[string][]linters.LinterOutput, error) {
	//format for tty format output
	var result = make(map[string][]linters.LinterOutput)
	lines := strings.Split(string(output), "\n")
	var filename string
	var location int
	fileErr := make(map[string]map[int][]string)
	re := regexp.MustCompile(`In (\S+) line (\d+):`)
	for _, line := range lines {
		if _, ok := fileErr[filename]; !ok {
			fileErr[filename] = make(map[int][]string)
		}
		if strings.HasPrefix(line, "In") {
			match := re.FindStringSubmatch(line)
			if len(match) >= 3 {
				filename = strings.TrimPrefix(match[1], "./")
				location, _ = strconv.Atoi(match[2])
			}

		} else if strings.HasPrefix(line, "For more information:") {
			break
		} else {
			fileErr[filename][location] = append(fileErr[filename][location], line)
		}
	}

	for filename, errs := range fileErr {
		for locationLine, msgs := range errs {
			sendMsg := strings.Join(msgs, "\n")
			sendMsg = "Is there some potential issue with your shell code?" + "\n```\n" + sendMsg + "\n```\n"
			addShellcheckOutput(result, filename, locationLine, sendMsg)
		}
	}

	return result, nil
}

func addShellcheckOutput(result map[string][]linters.LinterOutput, filename string, line int, message string) {
	output := &linters.LinterOutput{
		File:    filename,
		Line:    int(line),
		Column:  int(line),
		Message: message,
	}

	if outs, ok := result[output.File]; !ok {
		result[output.File] = []linters.LinterOutput{*output}
	} else {
		// remove duplicate
		var existed bool
		for _, v := range outs {
			if v.File == output.File && v.Line == output.Line && v.Column == output.Column && v.Message == output.Message {
				existed = true
				break
			}
		}

		if !existed {
			result[output.File] = append(result[output.File], *output)
		}
	}

}
