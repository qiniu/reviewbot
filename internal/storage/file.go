package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/qiniu/x/log"
)

type LocalStorage struct {
	mu sync.Mutex
}

func NewLocalStorage() Storage {
	return &LocalStorage{}
}

func (*LocalStorage) Writer(ctx context.Context, path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Errorf("failed to make log dir: %v", err)
	}
	dir, _ := os.Getwd()
	log.Infof("pwd: %s", dir)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	select {
	case <-ctx.Done():
		return fmt.Errorf("operation canceled: %w", ctx.Err())
	default:
		if _, err := file.Write(content); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	return nil
}

func (g *LocalStorage) Reader(ctx context.Context, path string) ([]byte, error) {
	filePath := path
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return data, nil
}
