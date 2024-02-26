package gofmt

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

var lintName = "gofmt"

func init() {
	linters.RegisterPullRequestHandler(lintName, gofmtHandler)
}

func gofmtHandler(log *xlog.Logger, a linters.Agent) error {

	if linters.IsEmpty(a.LinterConfig.Args...) {
		a.LinterConfig.Args = append([]string{}, "-d", "./")
	}

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

	return linters.Report(log, a, parsedOutput)
}

type Gofmt struct {
	dir     string
	gofmt   string
	execute func(dir, command string, args ...string) ([]byte, error)
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
		execute: func(dir, command string, args ...string) ([]byte, error) {
			c := exec.Command(command, args...)
			c.Dir = dir
			log.Printf("final command:  %v \n", c)
			return c.Output()
		},
	}, nil
}

func (g *Gofmt) Run(log *xlog.Logger, args ...string) ([]byte, error) {
	b, err := g.execute(g.dir, g.gofmt, args...)
	if err != nil {
		log.Errorf("gofmt run with status: %v, mark and continue", err)
		return b, err
	} else {
		log.Infof("gofmt succeeded")
	}
	return b, nil
}

func (g *Gofmt) Parse(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	return formatGofmtOutput(log, output)
}

//formatGofmtOutput formats the output of gofmt
//example:
// diff internal/linters/go/golangci-lint/golangci-lint.go.orig internal/linters/go/golangci-lint/golangci-lint.go
// --- internal/linters/go/golangci-lint/golangci-lint.go.orig
// +++ internal/linters/go/golangci-lint/golangci-lint.go
// @@ -17,7 +17,7 @@

//  var lintName = "golangci-lint"

// -               // test3333
// +// test3333
//  func init() {
//         linters.RegisterCodeReviewHandler(lintName, golangciLintHandler)
//  }
// @@ -33,7 +33,7 @@
//                 // turn off compile errors by default
//                 linterConfig.Args = append([]string{}, "-debug.no-compile-errors=true", "./...")
//         }
// -              //test 4444444
// +       //test 4444444
//         output, err := executor.Run(log, linterConfig.Args...)
//         if err != nil {
//                 log.Errorf("golangci-lint run failed: %v", err)

//output: map[file][]linters.LinterOutput
// map[internal/linters/go/golangci-lint/golangci-lint.go:[
//{internal/linters/go/golangci-lint/golangci-lint.go 24 7
// 	var lintName = "golangci-lint"

//    -               // test3333
//    +// test3333
// 	func init() {
// 		  linters.RegisterCodeReviewHandler(lintName, golangciLintHandler)
// 	}}
// {internal/linters/go/golangci-lint/golangci-lint.go 40 7
// 				  // turn off compile errors by default
// 				  linterConfig.Args = append([]string{}, "-debug.no-compile-errors=true", "./...")
// 		  }
//    -              //test 4444444
//    +      //test 4444444
// 		  output, err := executor.Run(log, linterConfig.Args...)
// 		  if err != nil {
// 				  log.Errorf("golangci-lint run failed: %v", err)
//    }]]

func formatGofmtOutput(log *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, error) {
	lines := strings.Split(string(output), "\n")
	var result = make(map[string][]linters.LinterOutput)
	fileErr := make(map[string][]string)
	var filename string
	for _, line := range lines {
		if strings.HasPrefix(line, "diff") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				filename = fields[2]
			}
			fileErr[filename] = []string{}
		} else if filename != "" {
			fileErr[filename] = append(fileErr[filename], line)
		}
	}

	for filename, errmsgs := range fileErr {
		var message string
		var locationLine, lineNumber int64
		var nonFirstTime bool
		for _, errmsg := range errmsgs {
			if strings.HasPrefix(errmsg, "@@") {
				//Add the previous set of error messages
				if nonFirstTime {
					message += "```"
					addGofmtOutput(result, filename, locationLine, lineNumber, message)
				}

				//Parameter Reset
				message = " Is your code not properly formatted? Here are some suggestions below\n```suggestion"
				locationLine = 0
				lineNumber = 0
				nonFirstTime = true

				//Extract row information
				re := regexp.MustCompile(`-(\d+),(\d+)`)
				match := re.FindStringSubmatch(errmsg)
				if len(match) > 2 {
					locationLine, _ = strconv.ParseInt(match[1], 10, 64)
					lineNumber, _ = strconv.ParseInt(match[2], 10, 64)
				}

			} else if strings.HasPrefix(errmsg, "+") {
				message += " \n " + strings.TrimLeft(errmsg, "+")
			}
		}
		nonFirstTime = false
		//Add once to the tail
		message += "\n```"
		addGofmtOutput(result, filename, locationLine, lineNumber, message)

	}

	return result, nil
}

func addGofmtOutput(result map[string][]linters.LinterOutput, filename string, locationLine, lineNumber int64, message string) {
	output := &linters.LinterOutput{
		File:      filename,
		Line:      int(locationLine + lineNumber - 1),
		Column:    int(lineNumber),
		Message:   message,
		StratLine: int(locationLine + 2),
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
