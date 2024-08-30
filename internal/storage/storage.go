package storage

import "context"

type Storage interface {
	Writer(ctx context.Context, path string, content []byte) error

	Reader(ctx context.Context, path string) ([]byte, error)
}
