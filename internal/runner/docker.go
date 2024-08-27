package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/x/log"
)

type DockerRunner struct {
	cli *client.Client
}

func NewDockerRunner() (Runner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerRunner{cli: cli}, nil
}

// Prepare will pull the docker image if it is not exist.
func (r *DockerRunner) Prepare(ctx context.Context, cfg *config.Linter) error {
	if cfg.DockerAsRunner == "" {
		return nil
	}

	_, _, err := r.cli.ImageInspectWithRaw(ctx, cfg.DockerAsRunner)
	if err == nil {
		return nil
	}
	if !client.IsErrNotFound(err) {
		return fmt.Errorf("failed to inspect image: %w", err)
	}

	reader, err := r.cli.ImagePull(ctx, cfg.DockerAsRunner, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// wait for image pull
	_, err = io.Copy(os.Stdout, reader)
	return err
}

func (r *DockerRunner) Run(ctx context.Context, cfg *config.Linter) (io.ReadCloser, error) {
	if cfg.DockerAsRunner == "" {
		return nil, fmt.Errorf("docker image is not set")
	}

	if err := r.Prepare(ctx, cfg); err != nil {
		return nil, err
	}

	var (
		dockerConfig = &container.Config{
			Image:      cfg.DockerAsRunner,
			Env:        cfg.Env,
			Entrypoint: []string{"/bin/sh", "-c"},
			WorkingDir: cfg.WorkDir,
		}
		dockerHostConfig = &container.HostConfig{
			Binds: []string{cfg.WorkDir + ":" + cfg.WorkDir},
		}
	)

	// add git config so that git can work in the container
	wrapperScript := fmt.Sprintf(`#!/bin/sh
		git config --global --add safe.directory %s
		%s %s
		`, cfg.WorkDir, strings.Join(cfg.Command, " "), strings.Join(cfg.Args, " "))

	dockerConfig.Cmd = []string{wrapperScript}

	resp, err := r.cli.ContainerCreate(ctx, dockerConfig, dockerHostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	log.Infof("container created: %v", resp.ID)
	if err := r.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	}

	return r.cli.ContainerLogs(ctx, resp.ID, logOptions)
}
