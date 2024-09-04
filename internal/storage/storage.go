package storage

import "context"

type Storage interface {
	// Write writes the content to the specified key.
	Write(ctx context.Context, key string, content []byte) error

	// Read reads the content from the specified key.
	Read(ctx context.Context, key string) ([]byte, error)
}

const (
	DefaultLogName = "log.txt"
)
