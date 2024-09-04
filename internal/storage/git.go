package storage

import (
	"context"
)

type GitStorage struct{}

func NewGitStorage() (Storage, error) {
	return &GitStorage{}, nil
}

func (g *GitStorage) Write(ctx context.Context, key string, content []byte) error {
	return nil
}

func (g *GitStorage) Read(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}
