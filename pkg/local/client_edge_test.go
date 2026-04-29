package local_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"digital.vasic.storage/pkg/local"
	"digital.vasic.storage/pkg/object"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) (*local.Client, string) {
	t.Helper()
	dir := t.TempDir()
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	c, err := local.NewClient(&local.Config{RootDir: dir}, logger)
	require.NoError(t, err)

	err = c.Connect(context.Background())
	require.NoError(t, err)

	return c, dir
}

func TestClient_PermissionDenied(t *testing.T) {
	t.Parallel()

	c, dir := newTestClient(t)
	defer c.Close()

	ctx := context.Background()

	// Create a bucket and make it read-only
	err := c.CreateBucket(ctx, object.BucketConfig{Name: "readonly"})
	require.NoError(t, err)

	bucketPath := filepath.Join(dir, "readonly")
	err = os.Chmod(bucketPath, 0o444)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chmod(bucketPath, 0o755) })

	// PutObject should fail with permission denied
	err = c.PutObject(ctx, "readonly", "test.txt", strings.NewReader("data"), 4)
	assert.Error(t, err)
}

func TestClient_ConcurrentWritesSameKey(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	err := c.CreateBucket(ctx, object.BucketConfig{Name: "concurrent"})
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := strings.Repeat("x", idx+1)
			_ = c.PutObject(ctx, "concurrent", "same-key.txt",
				strings.NewReader(data), int64(len(data)))
		}(i)
	}
	wg.Wait()

	// The file should exist (last writer wins)
	rc, err := c.GetObject(ctx, "concurrent", "same-key.txt")
	require.NoError(t, err)
	defer rc.Close()

	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.NotEmpty(t, body)
}

func TestClient_SymbolicLinks(t *testing.T) {
	t.Parallel()

	c, dir := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	err := c.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Write a real file
	err = c.PutObject(ctx, "bucket", "real.txt",
		strings.NewReader("real data"), 9)
	require.NoError(t, err)

	// Create a symlink to the real file
	realPath := filepath.Join(dir, "bucket", "real.txt")
	linkPath := filepath.Join(dir, "bucket", "link.txt")
	err = os.Symlink(realPath, linkPath)
	if err != nil {
		t.Skipf("symlinks not supported: %v", err)  // SKIP-OK: #legacy-skip-untriaged-2026-04-29
	}

	// GetObject on the symlink should work
	rc, err := c.GetObject(ctx, "bucket", "link.txt")
	require.NoError(t, err)
	defer rc.Close()

	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "real data", string(body))
}

func TestClient_EmptyKeyValue(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	err := c.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Empty value (zero-length body)
	err = c.PutObject(ctx, "bucket", "empty.txt",
		strings.NewReader(""), 0)
	require.NoError(t, err)

	rc, err := c.GetObject(ctx, "bucket", "empty.txt")
	require.NoError(t, err)
	defer rc.Close()

	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Empty(t, body)

	// Stat should show size 0
	info, err := c.StatObject(ctx, "bucket", "empty.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size)
}

func TestClient_KeyWithSpecialCharacters(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	err := c.CreateBucket(ctx, object.BucketConfig{Name: "special"})
	require.NoError(t, err)

	tests := []struct {
		name string
		key  string
	}{
		{"spaces", "file with spaces.txt"},
		{"unicode", "resume\u0301.txt"},
		{"nested_deep", "a/b/c/d/e/f.txt"},
		{"dots", "many...dots...file.txt"},
		{"hyphen_underscore", "my-file_v2.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := "content for " + tt.key
			err := c.PutObject(ctx, "special", tt.key,
				strings.NewReader(data), int64(len(data)))
			require.NoError(t, err)

			rc, err := c.GetObject(ctx, "special", tt.key)
			require.NoError(t, err)
			defer rc.Close()

			body, err := io.ReadAll(rc)
			require.NoError(t, err)
			assert.Equal(t, data, string(body))
		})
	}
}

func TestClient_VeryLargeValue(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	err := c.CreateBucket(ctx, object.BucketConfig{Name: "large"})
	require.NoError(t, err)

	// Write a 5MB object
	size := 5 * 1024 * 1024
	data := strings.Repeat("A", size)

	err = c.PutObject(ctx, "large", "big.bin",
		strings.NewReader(data), int64(size))
	require.NoError(t, err)

	info, err := c.StatObject(ctx, "large", "big.bin")
	require.NoError(t, err)
	assert.Equal(t, int64(size), info.Size)

	// Read it back
	rc, err := c.GetObject(ctx, "large", "big.bin")
	require.NoError(t, err)
	defer rc.Close()

	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Len(t, body, size)
}

func TestClient_OperationsWhileDisconnected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	c, err := local.NewClient(&local.Config{RootDir: dir}, logger)
	require.NoError(t, err)

	// Do NOT connect -- all operations should fail
	ctx := context.Background()

	err = c.CreateBucket(ctx, object.BucketConfig{Name: "b"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	_, err = c.ListBuckets(ctx)
	assert.Error(t, err)

	err = c.PutObject(ctx, "b", "k", strings.NewReader("x"), 1)
	assert.Error(t, err)

	_, err = c.GetObject(ctx, "b", "k")
	assert.Error(t, err)

	err = c.DeleteObject(ctx, "b", "k")
	assert.Error(t, err)

	_, err = c.ListObjects(ctx, "b", "")
	assert.Error(t, err)

	_, err = c.StatObject(ctx, "b", "k")
	assert.Error(t, err)

	err = c.CopyObject(ctx,
		object.ObjectRef{Bucket: "b", Key: "src"},
		object.ObjectRef{Bucket: "b", Key: "dst"})
	assert.Error(t, err)

	err = c.HealthCheck(ctx)
	assert.Error(t, err)

	_, err = c.BucketExists(ctx, "b")
	assert.Error(t, err)

	err = c.DeleteBucket(ctx, "b")
	assert.Error(t, err)
}

func TestClient_GetNonexistentObject(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	err := c.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	_, err = c.GetObject(ctx, "bucket", "nonexistent.txt")
	assert.Error(t, err)
}

func TestClient_DeleteNonexistentObject(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	err := c.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	err = c.DeleteObject(ctx, "bucket", "nonexistent.txt")
	assert.Error(t, err)
}

func TestClient_BucketExistsNonexistent(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	exists, err := c.BucketExists(ctx, "no-such-bucket")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestClient_ListObjectsEmptyBucket(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	err := c.CreateBucket(ctx, object.BucketConfig{Name: "empty"})
	require.NoError(t, err)

	objects, err := c.ListObjects(ctx, "empty", "")
	require.NoError(t, err)
	assert.Empty(t, objects)
}

func TestClient_PutObjectWithMetadata(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t)
	defer c.Close()

	ctx := context.Background()
	err := c.CreateBucket(ctx, object.BucketConfig{Name: "meta"})
	require.NoError(t, err)

	err = c.PutObject(ctx, "meta", "doc.txt",
		strings.NewReader("hello"), 5,
		object.WithContentType("text/plain"),
		object.WithMetadata(map[string]string{"author": "test"}))
	require.NoError(t, err)

	info, err := c.StatObject(ctx, "meta", "doc.txt")
	require.NoError(t, err)
	assert.Equal(t, "text/plain", info.ContentType)
	assert.Equal(t, "test", info.Metadata["author"])
}

func TestClient_NilConfig(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	_, err := local.NewClient(nil, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "root_dir is required")
}

func TestClient_EmptyRootDir(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	_, err := local.NewClient(&local.Config{RootDir: ""}, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "root_dir is required")
}

func TestClient_ConnectCloseIdempotent(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t)

	assert.True(t, c.IsConnected())

	err := c.Close()
	assert.NoError(t, err)
	assert.False(t, c.IsConnected())

	// Second close should not error
	err = c.Close()
	assert.NoError(t, err)
}
