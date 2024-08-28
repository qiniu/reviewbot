package storage

import (
	"context"

	"github.com/qiniu/reviewbot/config"
)

type GithubStorage struct {
}

func NewGitubStorage(cfg config.GithubConfig) Storage {
	return &GithubStorage{}
}

func (g *GithubStorage) Writer(ctx context.Context, path string, content []byte) error {
	return nil
}

func (g *GithubStorage) Reader(ctx context.Context, path string) (string, error) {
	return "", nil
}
