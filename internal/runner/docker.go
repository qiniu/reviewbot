package runner

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

	// apply the git config safe directory modifier
	cfg.Modifier = newGitConfigSafeDirModifier(cfg.Modifier)
	// apply the docker artifact modifier
	cfg.Modifier = newDockerArtifactModifier(cfg.Modifier)
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
			AutoRemove: false, // do not remove container after it exits
		}
	)

	log.Infof("Docker config: entrypoint: %v, cmd: %v, env: %v, working dir: %v, volume: %v",
		dockerConfig.Entrypoint, dockerConfig.Cmd, dockerConfig.Env, dockerConfig.WorkingDir, dockerHostConfig.Binds)

	resp, err := r.cli.ContainerCreate(ctx, dockerConfig, dockerHostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	log.Infof("container created: %v", resp.ID)

	// NOTE(Carl): do not know why mount volume does not work in DinD mode,
	// copy the code to container instead.
	if err := r.copyToContainer(ctx, resp.ID, cfg.WorkDir, filepath.Dir(cfg.WorkDir)); err != nil {
		return nil, fmt.Errorf("failed to copy code to container: %w", err)
	}

	if cfg.DockerAsRunner.CopyLinterFromOrigin {
		linterOriginPath, err := exec.LookPath(cfg.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to find %s :%w", cfg.Name, err)
		}

		err = r.copyToContainer(ctx, resp.ID, linterOriginPath, "/usr/local/bin/")
		if err != nil {
			return nil, fmt.Errorf("failed to copy linter to container: %w", err)
		}
	}

	if err := r.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}
	log.Infof("container started: %v", resp.ID)

	statusCh, errCh := r.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return nil, fmt.Errorf("error waiting for container: %w", err)
	case <-statusCh:
	}

	// find the artifact path
	var artifactPath string
	for _, env := range cfg.Env {
		if strings.HasPrefix(env, "ARTIFACT=") {
			artifactPath = strings.TrimPrefix(env, "ARTIFACT=")
			break
		}
	}

	if artifactPath != "" {
		return r.readArtifactContent(ctx, resp.ID, artifactPath)
	}

	// read the log from container
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
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

func (r *DockerRunner) copyToContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	// prepare source code dir
	srcInfo, err := archive.CopyInfoSourcePath(srcPath, false)
	if err != nil {
		return fmt.Errorf("failed to create archive info: %w", err)
	}

	srcArchive, err := archive.TarResource(srcInfo)
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}
	defer srcArchive.Close()

	err = r.cli.CopyToContainer(ctx, containerID, dstPath, srcArchive, container.CopyToContainerOptions{})
	if err != nil {
		return fmt.Errorf("failed to copy to container: %w", err)
	}

	return nil
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

type dockerArtifactModifier struct {
	next config.Modifier
}

// newDockerArtifactModifier creates a new docker artifact modifier.
// It will modify the linter config to support docker artifact.
func newDockerArtifactModifier(next config.Modifier) config.Modifier {
	return &dockerArtifactModifier{next: next}
}

// Modify modifies the linter config to support docker artifact.
func (d *dockerArtifactModifier) Modify(cfg *config.Linter) (*config.Linter, error) {
	base, err := d.next.Modify(cfg)
	if err != nil {
		return nil, err
	}

	newCfg := base

	// find the artifact path from original env
	hasArtifactEnv := false
	for _, env := range newCfg.Env {
		if strings.HasPrefix(env, "ARTIFACT=") {
			hasArtifactEnv = true
			break
		}
	}

	// if the artifact path is already set, just return
	if hasArtifactEnv {
		return newCfg, nil
	}

	// check if the artifact path is used in args
	usesArtifact := false
	for _, arg := range newCfg.Args {
		if strings.Contains(arg, "$ARTIFACT") || strings.Contains(arg, "${ARTIFACT}") {
			usesArtifact = true
			break
		}
	}

	// if the artifact path is not used in args, just return
	if !usesArtifact {
		return newCfg, nil
	}

	// if the artifact path is used in args, but not set in env, we add related config
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	randomString := hex.EncodeToString(randomBytes)
	artifactDir := "/tmp/artifacts-" + randomString
	newCfg.Env = append(newCfg.Env, "ARTIFACT="+artifactDir)

	// create the artifact dir
	createDirCmd := "mkdir -p " + artifactDir
	if len(newCfg.Args) > 0 {
		// insert the create dir command before the existing commands, use \n to separate
		newCfg.Args = append([]string{createDirCmd + "\n"}, newCfg.Args...)
	} else {
		// if there is no existing commands, just add the create dir command
		newCfg.Args = []string{createDirCmd}
	}

	return newCfg, nil
}

func (r *DockerRunner) readArtifactContent(ctx context.Context, containerID, artifactPath string) (io.ReadCloser, error) {
	log := lintersutil.FromContext(ctx)
	reader, _, err := r.cli.CopyFromContainer(ctx, containerID, artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to copy from container: %w", err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	var artifactContents []struct {
		name    string
		content bytes.Buffer
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading tar: %w", err)
		}

		// only read regular file
		if header.Typeflag == tar.TypeReg {
			var content bytes.Buffer
			if _, err := io.Copy(&content, tr); err != nil {
				return nil, fmt.Errorf("error copying file content: %w", err)
			}
			artifactContents = append(artifactContents, struct {
				name    string
				content bytes.Buffer
			}{name: header.Name, content: content})
		}
	}

	if len(artifactContents) == 0 {
		return nil, nil // return nil means the ARTIFACT dir is empty
	}

	// sort by file name
	sort.Slice(artifactContents, func(i, j int) bool {
		return artifactContents[i].name < artifactContents[j].name
	})

	// merge all file contents
	var combinedContent bytes.Buffer
	for _, ac := range artifactContents {
		log.Infof("artifact file: %s", ac.name)
		fmt.Fprintf(&combinedContent, "---%s---\n", ac.name)
		if _, err := io.Copy(&combinedContent, &ac.content); err != nil {
			return nil, fmt.Errorf("error combining file contents: %w", err)
		}
		combinedContent.WriteString("\n")
	}

	return io.NopCloser(bytes.NewReader(combinedContent.Bytes())), nil
}
