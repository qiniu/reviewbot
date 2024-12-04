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

package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/util"
)

// Runner defines the interface for executing linters.
// It is not concurrency-safe. Use Clone() to obtain a new instance for each linter when running concurrently.
type Runner interface {
	// Prepare prepares the linter for running.
	Prepare(ctx context.Context, cfg *config.Linter) error
	// Run runs the linter and returns the output.
	Run(ctx context.Context, cfg *config.Linter) (io.ReadCloser, error)
	// GetFinalScript returns the final script to be executed.
	// It should be called after Run function. and it's used for logging and debugging.
	GetFinalScript() string
	// Clone returns a new Runner instance with the same configuration to keep concurrency safe.
	// It's used for creating a new runner for each linter.
	Clone() Runner
}

// LocalRunner is a runner that runs the linter locally.
type LocalRunner struct {
	script string
}

func NewLocalRunner() Runner {
	return &LocalRunner{}
}

func (l *LocalRunner) Clone() Runner {
	return &LocalRunner{}
}

func (l *LocalRunner) GetFinalScript() string {
	return l.script
}

func (l *LocalRunner) Prepare(ctx context.Context, cfg *config.Linter) error {
	return nil
}

func (l *LocalRunner) Run(ctx context.Context, cfg *config.Linter) (io.ReadCloser, error) {
	log := util.FromContext(ctx)
	newCfg, err := cfg.Modifier.Modify(cfg)
	if err != nil {
		return nil, err
	}
	log.Infof("final config: %v", newCfg)

	// construct the script content
	scriptContent := "set -e\n"

	// handle command
	var shell []string
	if len(newCfg.Command) > 0 && (newCfg.Command[0] == "/bin/bash" || newCfg.Command[0] == "/bin/sh") {
		shell = newCfg.Command
	} else {
		shell = []string{"/bin/sh", "-c"}
		if len(newCfg.Command) > 0 {
			scriptContent += strings.Join(newCfg.Command, " ") + "\n"
		}
	}

	// handle args
	scriptContent += strings.Join(newCfg.Args, " ")

	log.Infof("Script content: \n%s", scriptContent)
	l.script = scriptContent

	//nolint:gosec
	c := exec.CommandContext(ctx, shell[0], append(shell[1:], scriptContent)...)
	c.Dir = newCfg.WorkDir

	// create a temp dir for the artifact
	artifact, err := os.MkdirTemp("", "artifact")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(artifact)
	c.Env = append(os.Environ(), fmt.Sprintf("ARTIFACT=%s", artifact))
	c.Env = append(c.Env, newCfg.Env...)

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

// for easy mock.
// copy from https://github.com/moby/moby/blob/v27.2.1/client/interface.go#L48.
type DockerClientInterface interface {
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
	CopyToContainer(ctx context.Context, containerID, dstPath string, content io.Reader, options container.CopyToContainerOptions) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	CopyFromContainer(ctx context.Context, containerID, srcPath string) (io.ReadCloser, container.PathStat, error)
	ContainerStatPath(ctx context.Context, containerID, path string) (container.PathStat, error)
}
