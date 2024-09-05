package runner_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockDockerClient struct {
	mock.Mock
}

var _ runner.DockerClientInterface = (*MockDockerClient)(nil)

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
			ctx := context.WithValue(context.Background(), config.EventGUIDKey, "test")
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
		name       string
		cfg        *config.Linter
		wantErr    bool
		wantOutput string
		testCopy   bool
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
			mockCli.On("ContainerLogs", mock.Anything, "test-container-id", mock.Anything).Return(mockLogs, nil)
			if tc.testCopy {
				err := os.WriteFile("/usr/local/bin/golangci-lint-1", []byte("test"), 0755)
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
				mockCli.On("CopyToContainer", mock.Anything, "test-container-id", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			}
			dr, err := runner.NewDockerRunner(mockCli)
			assert.NoError(t, err)
			ctx := context.WithValue(context.Background(), config.EventGUIDKey, "test")
			output, err := dr.Run(ctx, tc.cfg)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, output)
				defer output.Close()

				content, err := io.ReadAll(output)
				assert.NoError(t, err)
				assert.Equal(t, tc.wantOutput, string(content))
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
