package storage

import (
	"context"

	"github.com/qiniu/reviewbot/config"
)

type AWSStorage struct {
}

func NewAWSStorage(cfg config.AWSConfig) Storage {
	return &AWSStorage{}
}

func (g *AWSStorage) Writer(ctx context.Context, path string, content []byte) error {
	return nil
}

func (g *AWSStorage) Reader(ctx context.Context, path string) (string, error) {
	return "", nil
}
