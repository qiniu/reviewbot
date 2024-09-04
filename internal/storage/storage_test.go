package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/qiniu/reviewbot/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "file_storage_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	fs, err := storage.NewLocalStorage(tempDir)
	require.NoError(t, err)

	t.Run("Write and Read", func(t *testing.T) {
		ctx := context.Background()
		content := []byte("测试内容")
		path := "test.txt"
		err := fs.Write(ctx, path, content)
		require.NoError(t, err)
		readContent, err := fs.Read(ctx, path)
		require.NoError(t, err)
		assert.Equal(t, content, readContent)
	})

	t.Run("Read Non-existent File", func(t *testing.T) {
		ctx := context.Background()
		_, err := fs.Read(ctx, "non_existent.txt")
		require.Error(t, err)
	})

	t.Run("Write to Non-existent Directory", func(t *testing.T) {
		ctx := context.Background()
		content := []byte("测试内容")
		path := filepath.Join("non", "existent", "dir", "test.txt")

		err := fs.Write(ctx, path, content)
		require.NoError(t, err)

		readContent, err := fs.Read(ctx, path)
		require.NoError(t, err)
		require.Equal(t, content, readContent)
	})

	t.Run("Overwrite Existing File", func(t *testing.T) {
		ctx := context.Background()
		path := "overwrite.txt"
		err := fs.Write(ctx, path, []byte("原始内容"))
		require.NoError(t, err)

		newContent := []byte("新内容")
		err = fs.Write(ctx, path, newContent)
		require.NoError(t, err)

		readContent, err := fs.Read(ctx, path)
		require.NoError(t, err)
		require.Equal(t, newContent, readContent)
	})
}
