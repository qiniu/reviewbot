package gofmt

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

var lintName = "gofmt"

func init() {
	linters.RegisterPullRequestHandler(lintName, gofmtHandler)
	linters.RegisterLinterLanguages(lintName, []string{".go"})
}

func gofmtHandler(ctx context.Context, a linters.Agent) error {
	log := lintersutil.FromContext(ctx)
	if linters.IsEmpty(a.LinterConfig.Args...) {
		a.LinterConfig.Args = append([]string{}, "-d", "./")
	}

	// Since GitHub's check run feature does not have the suggestion functionality, GitHub PR review is fixed used to display gofmt reports.
	// Details: https://github.com/qiniu/reviewbot/issues/166
	a.LinterConfig.ReportFormat = config.GithubPRReview

	executor, err := NewgofmtExecutor(a.LinterConfig.WorkDir)
	if err != nil {
		log.Errorf("init gofmt executor failed: %v", err)
		return err
	}

	output, err := executor.Run(log, a.LinterConfig.Args...)
	if err != nil {
		log.Errorf("gofmt run failed: %v", err)
		return err
	}
	parsedOutput, err := executor.Parse(log, output)
	if err != nil {
		log.Errorf("gofmt parse output failed: %v", err)
		return err
	}

	return linters.Report(ctx, a, parsedOutput)
}

type Gofmt struct {
	dir     string
	gofmt   string
	execute func(dir, command string, args ...string) ([]byte, []byte, error)
}

func NewgofmtExecutor(dir string) (linters.Linter, error) {
	log.Infof("gofmt executor init")
	g, err := exec.LookPath("gofmt")
	if err != nil {
		return nil, err
	}
	return &Gofmt{
		dir:   dir,
		gofmt: g,
		execute: func(dir, command string, args ...string) ([]byte, []byte, error) {
			c := exec.Command(command, args...)
			c.Dir = dir
			log.Printf("final command:  %v \n", c)
			if c.Stdout != nil {
				return nil, nil, errors.New("exec: Stdout already set")
			}
			if c.Stderr != nil {
				return nil, nil, errors.New("exec: Stderr already set")
			}
			var stdoutBuffer bytes.Buffer
			var stderrBuffer bytes.Buffer
			c.Stdout = &stdoutBuffer
			c.Stderr = &stderrBuffer
			err := c.Run()
			return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), err
		},
	}, nil
}

func (g *Gofmt) Run(log *xlog.Logger, args ...string) ([]byte, error) {
	stdoutput, stderr, err := g.execute(g.dir, g.gofmt, args...)
	if err != nil {
		log.Warnf("gofmt run with status: %v, mark and continue", err)
		if stderr != nil {
			log.Warnf("gofmt run cause stderr: %s, mark and continue", stderr)
		}
		return stdoutput, nil
	} else {
		log.Infof("gofmt running succeeded")
	}
	return stdoutput, nil
}

func (g *Gofmt) Parse(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	log.Infof("gofmt output is being parsed")
	return formatGofmtOutput(output)
}

// Go Doc Comments Info: https://tip.golang.org/doc/comment
func formatGofmtOutput(output []byte) (map[string][]linters.LinterOutput, error) {
	result := make(map[string][]linters.LinterOutput)
	lines := strings.Split(string(output), "\n")
	// map[$filename]map[$diffname][]string
	fileErr := make(map[string]map[string][]string)
	// filename eg. test/test.go
	var filename string
	// diffname eg. @@ -19,5 +19,5 @@
	var diffname string
	for _, line := range lines {
		if strings.HasPrefix(line, "diff") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				filename = fields[2]
			}
			fileErr[filename] = map[string][]string{}
		} else if filename != "" {
			if strings.HasPrefix(line, "@@") {
				diffname = line
				fileErr[filename][diffname] = []string{}

			} else if diffname != "" && !strings.HasPrefix(line, "-") {
				fileErr[filename][diffname] = append(fileErr[filename][diffname], line)
			}
		}
	}

	type eachDiffInfo struct {
		// diffLineCount is means that how many diff lines
		diffLineCount int
		// firstDiffLine refers to the first line of  diff lines
		firstDiffLine int
		// fixedMessage refers to the diff fixed lines
		fixedMessage string
		// tmpLine refers to the line immediately following the blank line.
		tmpLine string
	}

	regexpDiff := regexp.MustCompile(`-(\d+),(\d+)`)
	for filename, errmsgs := range fileErr {
		for diffname, errmsg := range errmsgs {
			//"diffStartLine" refers to the starting line of a diff context, indicating the beginning position of a diff block.
			var diffStartLine int64
			match := regexpDiff.FindStringSubmatch(diffname)
			if len(match) > 2 {
				diffStartLine, _ = strconv.ParseInt(match[1], 10, 64)
			}

			var currentInfo eachDiffInfo
			for i, line := range errmsg {
				if strings.HasPrefix(line, "+") {
					if currentInfo.firstDiffLine == 0 {
						currentInfo.firstDiffLine = i + 1
					}
					currentInfo.diffLineCount++
					currentInfo.fixedMessage += strings.TrimLeft(line, "+") + "\n"
					if strings.TrimLeft(line, "+") == "" && i+1 < len(errmsg) {
						// If trimLeft line is only a blank line, temporarily store tmpLine.
						currentInfo.tmpLine = errmsg[i+1]
					}
				} else {
					if currentInfo.fixedMessage != "" {
						// If fixedMessage is either just a blank line or ends with a blank line.
						// In order to display the GitHub suggestion comment correctly, it should be appended with tmpLine.
						if currentInfo.fixedMessage == "\n" || strings.HasSuffix(currentInfo.fixedMessage, "\n\n") {
							currentInfo.fixedMessage += currentInfo.tmpLine + "\n"
						}
						finalMessage := " Is your code not properly formatted? Here are some suggestions below\n```suggestion\n" + currentInfo.fixedMessage + "```"
						addGofmtOutput(result, filename, diffStartLine, int64(currentInfo.firstDiffLine), int64(currentInfo.diffLineCount), finalMessage)
					}
					currentInfo = eachDiffInfo{}
				}
			}
		}
	}

	return result, nil
}

func addGofmtOutput(result map[string][]linters.LinterOutput, filename string, diffStartLine, firstDiffLine, diffLineCount int64, message string) {
	var output *linters.LinterOutput
	// If diffLineCount==1, set a single-line comment on GitHub; otherwise, set a multi-line comment.
	if diffLineCount == 1 {
		output = &linters.LinterOutput{
			File:    filename,
			Line:    int(diffStartLine + firstDiffLine - 1),
			Column:  int(diffLineCount),
			Message: message,
		}
	} else {
		output = &linters.LinterOutput{
			File:      filename,
			Line:      int(diffStartLine + firstDiffLine + diffLineCount - 2),
			Column:    int(diffLineCount),
			Message:   message,
			StartLine: int(diffStartLine + firstDiffLine - 1),
		}
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
