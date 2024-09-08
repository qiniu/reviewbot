package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/lintersutil"
)

type DockerRunner struct {
	cli    DockerClientInterface
	script string
}

func NewDockerRunner(cli DockerClientInterface) (Runner, error) {
	if cli == nil {
		var err error
		cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, err
		}
	}
	return &DockerRunner{cli: cli}, nil
}

func (r *DockerRunner) GetFinalScript() string {
	return r.script
}

// Prepare will pull the docker image if it is not exist.
func (r *DockerRunner) Prepare(ctx context.Context, cfg *config.Linter) error {
	if cfg.DockerAsRunner.Image == "" {
		return nil
	}

	_, _, err := r.cli.ImageInspectWithRaw(ctx, cfg.DockerAsRunner.Image)
	if err == nil {
		return nil
	}
	if !client.IsErrNotFound(err) {
		return fmt.Errorf("failed to inspect image: %w", err)
	}

	reader, err := r.cli.ImagePull(ctx, cfg.DockerAsRunner.Image, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// wait for image pull
	_, err = io.Copy(os.Stdout, reader)
	return err
}

func (r *DockerRunner) Run(ctx context.Context, cfg *config.Linter) (io.ReadCloser, error) {
	log := lintersutil.FromContext(ctx)
	if cfg.DockerAsRunner.Image == "" {
		return nil, fmt.Errorf("docker image is not set")
	}

	if err := r.Prepare(ctx, cfg); err != nil {
		return nil, err
	}

	cfg.Modifier = newGitConfigSafeDirModifier(cfg.Modifier)
	cfg, err := cfg.Modifier.Modify(cfg)
	if err != nil {
		return nil, err
	}

	// construct the script content
	scriptContent := "set -e\n"

	// determine the entrypoint
	var entrypoint []string
	if len(cfg.Command) > 0 && (cfg.Command[0] == "/bin/bash" || cfg.Command[0] == "/bin/sh") {
		entrypoint = cfg.Command // 使用 cfg 中指定的 shell
	} else {
		entrypoint = []string{"/bin/sh", "-c"}
		if len(cfg.Command) > 0 {
			scriptContent += strings.Join(cfg.Command, " ") + "\n"
		}
	}

	// handle args
	scriptContent += strings.Join(cfg.Args, " ")
	log.Infof("Script content: \n%s", scriptContent)
	r.script = scriptContent

	var (
		dockerConfig = &container.Config{
			Image:      cfg.DockerAsRunner.Image,
			Env:        cfg.Env,
			Entrypoint: entrypoint,
			Cmd:        []string{scriptContent},
			WorkingDir: cfg.WorkDir,
		}
		dockerHostConfig = &container.HostConfig{
			Binds: []string{cfg.WorkDir + ":" + cfg.WorkDir},
		}
	)

	log.Infof("Docker config: entrypoint: %v, cmd: %v, env: %v, working dir: %v",
		dockerConfig.Entrypoint, dockerConfig.Cmd, dockerConfig.Env, dockerConfig.WorkingDir)

	// TODO(Carl): support artifact env?
	resp, err := r.cli.ContainerCreate(ctx, dockerConfig, dockerHostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	log.Infof("container created: %v", resp.ID)

	if cfg.DockerAsRunner.CopyLinterFromOrigin {
		linterOrignPath, err := exec.LookPath(cfg.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to find %s :%w", cfg.Name, err)
		}

		reader, err := archive.Tar(linterOrignPath, archive.Uncompressed)
		if err != nil {
			return nil, fmt.Errorf("failed to create tar reader: %w", err)
		}

		err = r.cli.CopyToContainer(ctx, resp.ID, "/usr/local/bin/", reader, container.CopyToContainerOptions{})
		if err != nil {
			return nil, fmt.Errorf("copy to containner was failed : %w", err)
		}
	}

	if err := r.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}
	log.Infof("container started: %v", resp.ID)

	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
		Details:    false,
		Tail:       "all",
	}

	logReader, err := r.cli.ContainerLogs(ctx, resp.ID, logOptions)
	if err != nil {
		return nil, err
	}

	// remove docker log header
	cleanReader := NewCleanLogReader(logReader)
	return cleanReader, nil
}

type CleanLogReader struct {
	reader io.ReadCloser
	buffer *bufio.Reader
}

func NewCleanLogReader(reader io.ReadCloser) *CleanLogReader {
	return &CleanLogReader{
		reader: reader,
		buffer: bufio.NewReader(reader),
	}
}

func (c *CleanLogReader) Read(p []byte) (int, error) {
	line, err := c.buffer.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return 0, err
	}

	if len(line) > 8 {
		line = line[8:]
	}

	n := copy(p, line)
	return n, err
}

func (c *CleanLogReader) Close() error {
	c.buffer = nil
	return c.reader.Close()
}

type gitConfigSafeDirModifier struct {
	next config.Modifier
}

func newGitConfigSafeDirModifier(next config.Modifier) config.Modifier {
	return &gitConfigSafeDirModifier{next: next}
}

func (g *gitConfigSafeDirModifier) Modify(cfg *config.Linter) (*config.Linter, error) {
	base, err := g.next.Modify(cfg)
	if err != nil {
		return nil, err
	}

	newCfg := base
	args := []string{}
	// add safe.directory to git config
	args = append(args, fmt.Sprintf("git config --global --add safe.directory %s \n", cfg.WorkDir))
	args = append(args, base.Args...)
	newCfg.Args = args
	return newCfg, nil
}
