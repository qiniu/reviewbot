package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/qiniu/x/log"
)

type LocalStorage struct {
	rootDir string
}

func NewLocalStorage(rootDir string) (Storage, error) {
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to make log dir: %w", err)
	}
	return &LocalStorage{rootDir: rootDir}, nil
}

func (l *LocalStorage) Write(ctx context.Context, path string, content []byte) error {
	logFile := filepath.Join(l.rootDir, path, DefaultLogName)
	log.Infof("writing log to %s", logFile)
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		log.Errorf("failed to make log dir: %v", err)
	}

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
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

func (l *LocalStorage) Read(ctx context.Context, path string) ([]byte, error) {
	filePath := filepath.Join(l.rootDir, path, DefaultLogName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("operation canceled: %w", ctx.Err())
	default:
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("error reading file: %w", err)
		}
		return data, nil
	}
}
