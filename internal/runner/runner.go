package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/x/log"
)

type Runner interface {
	Prepare(ctx context.Context, cfg *config.Linter) error
	Run(ctx context.Context, cfg *config.Linter) (io.ReadCloser, error)
}

// LocalRunner is a runner that runs the linter locally.
type LocalRunner struct{}

func NewLocalRunner() Runner {
	return &LocalRunner{}
}

func (*LocalRunner) Prepare(ctx context.Context, cfg *config.Linter) error {
	return nil
}

func (*LocalRunner) Run(ctx context.Context, cfg *config.Linter) (io.ReadCloser, error) {
	command := cfg.Command
	executable := command[0]
	var cmdArgs []string
	if len(command) > 1 {
		cmdArgs = command[1:]
	}
	cmdArgs = append(cmdArgs, cfg.Args...)
	c := exec.Command(executable, cmdArgs...)
	c.Dir = cfg.WorkDir

	// create a temp dir for the artifact
	artifact, err := os.MkdirTemp("", "artifact")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(artifact)
	c.Env = append(os.Environ(), fmt.Sprintf("ARTIFACT=%s", artifact))
	c.Env = append(c.Env, cfg.Env...)

	log.Infof("run command: %v, workDir: %v", c, c.Dir)
	output, execErr := c.CombinedOutput()

	// read all files under the artifact dir
	var fileContent []byte
	artifactFiles, err := os.ReadDir(artifact)
	if err != nil {
		return nil, err
	}

	var idx int
	for _, file := range artifactFiles {
		if file.IsDir() {
			continue
		}
		log.Infof("artifact file: %v", file.Name())
		content, err := os.ReadFile(fmt.Sprintf("%s/%s", artifact, file.Name()))
		if err != nil {
			return nil, err
		}
		if len(content) == 0 {
			continue
		}
		if idx > 0 {
			fileContent = append(fileContent, '\n')
		}
		fileContent = append(fileContent, content...)
		idx++
	}

	// use the content of the files under Artifact dir as first priority
	if len(fileContent) > 0 {
		log.Debugf("artifact files used instead. legacy output:\n%v, now:\n%v", string(output), string(fileContent))
		output = fileContent
	}

	// wrap the output to io.ReadCloser
	return io.NopCloser(bytes.NewReader(output)), execErr
}
