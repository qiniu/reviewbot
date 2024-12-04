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

package runner_test

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/archive"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/runner"
	"github.com/qiniu/reviewbot/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLocalRunner(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.Linter
		wantErr    bool
		wantOutput string
	}{
		{
			name: "basic command execution",
			cfg: &config.Linter{
				Command:  []string{"echo"},
				Args:     []string{"hello"},
				Modifier: config.NewBaseModifier(),
			},
			wantErr:    false,
			wantOutput: "hello\n",
		},
		{
			name: "custom command execution",
			cfg: &config.Linter{
				Command:  []string{"/bin/sh", "-c"},
				Args:     []string{"echo hello"},
				Modifier: config.NewBaseModifier(),
			},
			wantErr:    false,
			wantOutput: "hello\n",
		},
		{
			name: "sh command execution",
			cfg: &config.Linter{
				Command:  []string{"sh", "-c"},
				Args:     []string{"echo hello"},
				Modifier: config.NewBaseModifier(),
			},
			wantErr:    false,
			wantOutput: "hello\n",
		},
		{
			name: "with artifact",
			cfg: &config.Linter{
				Command:  []string{"sh", "-c"},
				Args:     []string{"echo hello_world >> $ARTIFACT/output.txt"},
				Modifier: config.NewBaseModifier(),
			},
			wantErr:    false,
			wantOutput: "hello_world\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lr := runner.NewLocalRunner()
			ctx := context.WithValue(context.Background(), util.EventGUIDKey, "test")
			output, err := lr.Run(ctx, tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, output)
				defer output.Close()

				content, err := io.ReadAll(output)
				assert.NoError(t, err)
				assert.Equal(t, tt.wantOutput, string(content))
			}
		})
	}
}

func TestDockerRunner(t *testing.T) {
	tcs := []struct {
		name         string
		cfg          *config.Linter
		wantErr      bool
		wantOutput   string
		testCopy     bool
		withArtifact bool
	}{
		{
			name: "basic command execution",
			cfg: &config.Linter{
				DockerAsRunner: config.DockerAsRunner{
					Image:                "alpine:latest",
					CopyLinterFromOrigin: false,
				},

				Command:  []string{"echo"},
				Args:     []string{"hello"},
				Modifier: config.NewBaseModifier(),
			},
			wantErr:    false,
			wantOutput: "hello\n",
		},
		{
			name: "custom command execution",
			cfg: &config.Linter{
				DockerAsRunner: config.DockerAsRunner{
					Image:                "alpine:latest",
					CopyLinterFromOrigin: false,
				},
				Command:  []string{"/bin/sh", "-c"},
				Args:     []string{"echo hello"},
				Modifier: config.NewBaseModifier(),
			},
			wantErr:    false,
			wantOutput: "hello\n",
		},
		{
			name: "sh command execution",
			cfg: &config.Linter{
				DockerAsRunner: config.DockerAsRunner{
					Image:                "alpine:latest",
					CopyLinterFromOrigin: false,
				},
				Command:  []string{"sh", "-c"},
				Args:     []string{"echo hello"},
				Modifier: config.NewBaseModifier(),
			},
			wantErr:    false,
			wantOutput: "hello\n",
		},
		{
			name: "copy func test",
			cfg: &config.Linter{
				DockerAsRunner: config.DockerAsRunner{
					Image:                "alpine:latest",
					CopyLinterFromOrigin: true,
				},
				Command:  []string{"sh", "-c"},
				Args:     []string{"echo hello"},
				Modifier: config.NewBaseModifier(),
				Name:     "golangci-lint-1",
			},
			wantErr:    false,
			wantOutput: "hello\n",
			testCopy:   true,
		},
		{
			name: "with artifact",
			cfg: &config.Linter{
				DockerAsRunner: config.DockerAsRunner{
					Image:                "alpine:latest",
					CopyLinterFromOrigin: true,
				},
				Command:  []string{"sh", "-c"},
				Args:     []string{"echo hello_world > $ARTIFACT/output.txt"},
				Modifier: config.NewBaseModifier(),
				Name:     "golangci-lint-1",
			},
			wantErr:      false,
			testCopy:     true,
			wantOutput:   "---output.txt---\nhello_world\n",
			withArtifact: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// mock docker client
			mockCli := new(MockDockerClient)
			mockCli.On("ImageInspectWithRaw", mock.Anything, mock.Anything).Return(types.ImageInspect{}, []byte{}, nil)
			mockCli.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(container.CreateResponse{ID: "test-container-id"}, nil)
			mockCli.On("ContainerStart", mock.Anything, "test-container-id", mock.Anything).Return(nil)
			mockLogs := &mockReadCloser{
				Reader: strings.NewReader(tc.wantOutput),
			}
			waitRespCh := make(chan container.WaitResponse, 1)
			errCh := make(chan error, 1)
			mockCli.On("ContainerWait", mock.Anything, "test-container-id", mock.Anything).Return(waitRespCh, errCh)
			go func() {
				waitRespCh <- container.WaitResponse{StatusCode: 0}
			}()
			mockCli.On("CopyToContainer", mock.Anything, "test-container-id", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			if tc.testCopy {
				err := os.WriteFile("/usr/local/bin/golangci-lint-1", []byte("test"), 0o755)
				if err != nil {
					t.Errorf("Error writing to file: %v", err)
					return
				}

				defer func() {
					err = os.RemoveAll("/usr/local/bin/a.txt")
					if err != nil {
						t.Errorf("Error writing to file: %v", err)
						return
					}
				}()
			}
			if tc.withArtifact {
				var buf bytes.Buffer
				tarWriter := tar.NewWriter(&buf)
				content := []byte("hello_world")
				hdr := &tar.Header{
					Name:     "output.txt",
					Mode:     0600,
					Size:     int64(len(content)),
					ModTime:  time.Now(),
					Typeflag: tar.TypeReg,
				}
				if err := tarWriter.WriteHeader(hdr); err != nil {
					t.Fatalf("Could not write tar header: %v", err)
				}
				if _, err := tarWriter.Write(content); err != nil {
					t.Fatalf("Could not write tar content: %v", err)
				}
				if err := tarWriter.Close(); err != nil {
					t.Fatalf("Could not close tar writer: %v", err)
				}
				mockReader := io.NopCloser(bytes.NewReader(buf.Bytes()))
				mockCli.On("CopyFromContainer", mock.Anything, "test-container-id", mock.Anything).Return(mockReader, container.PathStat{}, nil)
			} else {
				mockCli.On("ContainerLogs", mock.Anything, "test-container-id", mock.Anything).Return(mockLogs, nil)
			}

			// mock archive wrapper
			mockArchiveWrapper := new(MockArchiveWrapper)
			mockArchiveWrapper.On("CopyInfoSourcePath", mock.Anything, mock.Anything).Return(archive.CopyInfo{}, nil)
			mockArchiveWrapper.On("TarResource", mock.Anything).Return(io.NopCloser(bytes.NewReader([]byte{})), nil)
			mockArchiveWrapper.On("PrepareArchiveCopy", mock.Anything, mock.Anything, mock.Anything).Return("dstDir", io.NopCloser(bytes.NewReader([]byte{})), nil)

			dr := runner.DockerRunner{
				Cli:            mockCli,
				ArchiveWrapper: mockArchiveWrapper,
			}

			ctx := context.WithValue(context.Background(), util.EventGUIDKey, "test")
			output, err := dr.Run(ctx, tc.cfg)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, output)
				defer output.Close()

				content, err := io.ReadAll(output)
				require.NoError(t, err)
				require.Equal(t, tc.wantOutput, string(content))
			}

			// assert docker client expectations
			mockCli.AssertExpectations(t)
		})
	}
}

type mockReadCloser struct {
	io.Reader
}

func (m *mockReadCloser) Close() error {
	return nil
}

type MockDockerClient struct {
	mock.Mock
}

var _ runner.DockerClientInterface = (*MockDockerClient)(nil)

func (m *MockDockerClient) ContainerStatPath(ctx context.Context, containerID, path string) (container.PathStat, error) {
	args := m.Called(ctx, containerID, path)
	return args.Get(0).(container.PathStat), args.Error(1)
}

func (m *MockDockerClient) CopyFromContainer(ctx context.Context, containerID, srcPath string) (io.ReadCloser, container.PathStat, error) {
	args := m.Called(ctx, containerID, srcPath)
	return args.Get(0).(io.ReadCloser), args.Get(1).(container.PathStat), args.Error(2)
}

func (m *MockDockerClient) ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	args := m.Called(ctx, containerID, condition)
	return args.Get(0).(chan container.WaitResponse), args.Get(1).(chan error)
}

func (m *MockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	args := m.Called(ctx, imageID)
	return args.Get(0).(types.ImageInspect), args.Get(1).([]byte), args.Error(2)
}

func (m *MockDockerClient) ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, refStr, options)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	args := m.Called(ctx, config, hostConfig, networkingConfig, platform, containerName)
	return args.Get(0).(container.CreateResponse), args.Error(1)
}

func (m *MockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerClient) ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, container, options)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockDockerClient) CopyToContainer(ctx context.Context, containerID, dstPath string, content io.Reader, options container.CopyToContainerOptions) error {
	m.Called(ctx, containerID, dstPath, content, options)
	return nil
}

type MockArchiveWrapper struct {
	mock.Mock
}

func (m *MockArchiveWrapper) CopyInfoSourcePath(path string, followLink bool) (archive.CopyInfo, error) {
	args := m.Called(path, followLink)
	return args.Get(0).(archive.CopyInfo), args.Error(1)
}

func (m *MockArchiveWrapper) TarResource(srcInfo archive.CopyInfo) (io.ReadCloser, error) {
	args := m.Called(srcInfo)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockArchiveWrapper) PrepareArchiveCopy(srcArchive io.Reader, srcInfo, dstInfo archive.CopyInfo) (dstDir string, preparedArchive io.ReadCloser, err error) {
	args := m.Called(srcArchive, srcInfo, dstInfo)
	return args.Get(0).(string), args.Get(1).(io.ReadCloser), args.Error(2)
}
