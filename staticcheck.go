package main

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/qiniu/x/xlog"
)

// executor knows how to execute staticcheck commands
type executor interface {
	Run(log *xlog.Logger, args ...string) ([]byte, error)
}

type staticcheck struct {
	// dir is the location of this repo.
	dir string
	// staticcheck is the path to the staticcheck binary.
	staticcheck string
	// execute executes a command
	execute func(dir, command string, args ...string) ([]byte, error)
}

// NewStaticcheckExecutor returns a new executor that knows how to execute staticcheck commands
// TODO: with config
func NewStaticcheckExecutor(dir string) (executor, error) {
	g, err := exec.LookPath("staticcheck")
	if err != nil {
		return nil, err
	}
	return &staticcheck{
		dir:         dir,
		staticcheck: g,
		execute: func(dir, command string, args ...string) ([]byte, error) {
			c := exec.Command(command, args...)
			c.Dir = dir
			return c.CombinedOutput()
		},
	}, nil
}

func (e *staticcheck) Run(log *xlog.Logger, args ...string) ([]byte, error) {
	b, err := e.execute(e.dir, e.staticcheck, args...)
	if err != nil {
		log.Errorf("staticcheck failed: %v", err)
	} else {
		log.Infof("staticcheck succeeded")
	}

	return b, err
}

type StaticcheckOutput struct {
	// file is the file name
	file string
	// line is the line number
	line string
	// column is the column number
	column string
	// message is the staticcheck message
	message string
	// code is the staticcheck code
	code string
}

// formatStaticcheckOutput formats the output of staticcheck
// example：
// gslb-api/utils.go:149:6: func dealWithRecordVipsId is unused (U1000)
// gslb-api/utils.go:162:2: this value of err is never used (SA4006)
// domain/repo/image.go:70:7: receiver name should be a reflection of its identity; don't use generic names such as "this" or "self" (ST1006)
//
//	output := map[string]string{
//		"gslb-api/utils.go": "gslb-api/utils.go:149:6: func dealWithRecordVipsId is unused (U1000)" + "\n" + "gslb-api/utils.go:162:2: this value of err is never used (SA4006)"
//		"domain/repo/image.go":"domain/repo/image.go:70:7: receiver name should be a reflection of its identity; don't use generic names such as "this" or "self" (ST1006)"
//	}
func formatStaticcheckOutput(output []byte) map[string][]StaticcheckOutput {
	lines := strings.Split(string(output), "\n")

	var result = make(map[string][]StaticcheckOutput)
	for _, line := range lines {
		if line == "" {
			continue
		}
		output := formatStaticcheckline(line)
		if output == nil {
			continue
		}

		result[output.file] = append(result[output.file], *output)
	}

	return result
}

func formatStaticcheckline(line string) *StaticcheckOutput {
	pattern := `^(.*):(\d+):(\d+): (.*) \((.*)\)$`
	regex := regexp.MustCompile(pattern)
	matches := regex.FindStringSubmatch(line)
	if len(matches) != 6 {
		return nil
	}

	return &StaticcheckOutput{
		file:    matches[1],
		line:    matches[2],
		column:  matches[3],
		message: matches[4],
		code:    matches[5],
	}
}
