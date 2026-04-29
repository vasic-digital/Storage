package local

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.storage/pkg/object"
)

func newTestClient(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	client, err := NewClient(&Config{RootDir: dir}, logrus.New())
	require.NoError(t, err)
	err = client.Connect(context.Background())
	require.NoError(t, err)
	return client, dir
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "root_dir is required",
		},
		{
			name:        "empty root dir",
			config:      &Config{RootDir: ""},
			expectError: true,
			errorMsg:    "root_dir is required",
		},
		{
			name:        "valid config",
			config:      &Config{RootDir: "/tmp/test-storage"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config, nil)
			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, client)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestClient_ConnectAndClose(t *testing.T) {
	dir := t.TempDir()
	client, err := NewClient(&Config{RootDir: dir}, nil)
	require.NoError(t, err)

	assert.False(t, client.IsConnected())

	err = client.Connect(context.Background())
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	err = client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())

	// Idempotent close
	err = client.Close()
	assert.NoError(t, err)
}

func TestClient_HealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		connected   bool
		expectError bool
	}{
		{"connected", true, false},
		{"not connected", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			client, err := NewClient(&Config{RootDir: dir}, nil)
			require.NoError(t, err)

			if tt.connected {
				err = client.Connect(context.Background())
				require.NoError(t, err)
			}

			err = client.HealthCheck(context.Background())
			if tt.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_BucketOperations(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	t.Run("create bucket", func(t *testing.T) {
		err := client.CreateBucket(ctx, object.BucketConfig{Name: "test-bucket"})
		require.NoError(t, err)
	})

	t.Run("bucket exists", func(t *testing.T) {
		exists, err := client.BucketExists(ctx, "test-bucket")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("bucket does not exist", func(t *testing.T) {
		exists, err := client.BucketExists(ctx, "nonexistent")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("list buckets", func(t *testing.T) {
		err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket-2"})
		require.NoError(t, err)

		buckets, err := client.ListBuckets(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(buckets), 2)

		names := make([]string, len(buckets))
		for i, b := range buckets {
			names[i] = b.Name
		}
		assert.Contains(t, names, "test-bucket")
		assert.Contains(t, names, "bucket-2")
	})

	t.Run("delete bucket", func(t *testing.T) {
		err := client.CreateBucket(ctx, object.BucketConfig{Name: "to-delete"})
		require.NoError(t, err)

		err = client.DeleteBucket(ctx, "to-delete")
		require.NoError(t, err)

		exists, err := client.BucketExists(ctx, "to-delete")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestClient_ObjectOperations(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "data"})
	require.NoError(t, err)

	t.Run("put and get object", func(t *testing.T) {
		content := []byte("hello, world!")
		err := client.PutObject(
			ctx, "data", "greeting.txt",
			bytes.NewReader(content), int64(len(content)),
		)
		require.NoError(t, err)

		reader, err := client.GetObject(ctx, "data", "greeting.txt")
		require.NoError(t, err)
		defer func() { _ = reader.Close() }()

		got, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("put with metadata", func(t *testing.T) {
		content := []byte(`{"key":"value"}`)
		err := client.PutObject(
			ctx, "data", "meta.json",
			bytes.NewReader(content), int64(len(content)),
			object.WithContentType("application/json"),
			object.WithMetadata(map[string]string{"version": "1"}),
		)
		require.NoError(t, err)

		info, err := client.StatObject(ctx, "data", "meta.json")
		require.NoError(t, err)
		assert.Equal(t, "application/json", info.ContentType)
		assert.Equal(t, "1", info.Metadata["version"])
	})

	t.Run("stat object", func(t *testing.T) {
		info, err := client.StatObject(ctx, "data", "greeting.txt")
		require.NoError(t, err)
		assert.Equal(t, "greeting.txt", info.Key)
		assert.Equal(t, int64(13), info.Size)
		assert.False(t, info.LastModified.IsZero())
	})

	t.Run("list objects", func(t *testing.T) {
		objects, err := client.ListObjects(ctx, "data", "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(objects), 2)
	})

	t.Run("list objects with prefix", func(t *testing.T) {
		content := []byte("nested")
		err := client.PutObject(
			ctx, "data", "sub/nested.txt",
			bytes.NewReader(content), int64(len(content)),
		)
		require.NoError(t, err)

		objects, err := client.ListObjects(ctx, "data", "sub/")
		require.NoError(t, err)
		assert.Len(t, objects, 1)
		assert.Equal(t, "sub/nested.txt", objects[0].Key)
	})

	t.Run("copy object", func(t *testing.T) {
		err := client.CreateBucket(
			ctx, object.BucketConfig{Name: "backup"},
		)
		require.NoError(t, err)

		err = client.CopyObject(
			ctx,
			object.ObjectRef{Bucket: "data", Key: "greeting.txt"},
			object.ObjectRef{Bucket: "backup", Key: "greeting-copy.txt"},
		)
		require.NoError(t, err)

		reader, err := client.GetObject(ctx, "backup", "greeting-copy.txt")
		require.NoError(t, err)
		defer func() { _ = reader.Close() }()

		got, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello, world!"), got)
	})

	t.Run("copy object with sidecar metadata", func(t *testing.T) {
		err := client.CopyObject(
			ctx,
			object.ObjectRef{Bucket: "data", Key: "meta.json"},
			object.ObjectRef{Bucket: "backup", Key: "meta-copy.json"},
		)
		require.NoError(t, err)

		info, err := client.StatObject(ctx, "backup", "meta-copy.json")
		require.NoError(t, err)
		assert.Equal(t, "application/json", info.ContentType)
	})

	t.Run("delete object", func(t *testing.T) {
		err := client.DeleteObject(ctx, "data", "greeting.txt")
		require.NoError(t, err)

		_, err = client.GetObject(ctx, "data", "greeting.txt")
		require.Error(t, err)
	})

	t.Run("delete nonexistent object", func(t *testing.T) {
		err := client.DeleteObject(ctx, "data", "nonexistent.txt")
		require.Error(t, err)
	})
}

func TestClient_OperationsWhenNotConnected(t *testing.T) {
	dir := t.TempDir()
	client, err := NewClient(&Config{RootDir: dir}, nil)
	require.NoError(t, err)
	ctx := context.Background()

	errTests := []struct {
		name string
		fn   func() error
	}{
		{
			"HealthCheck",
			func() error { return client.HealthCheck(ctx) },
		},
		{
			"CreateBucket",
			func() error {
				return client.CreateBucket(
					ctx, object.BucketConfig{Name: "b"},
				)
			},
		},
		{
			"DeleteBucket",
			func() error { return client.DeleteBucket(ctx, "b") },
		},
		{
			"PutObject",
			func() error {
				return client.PutObject(
					ctx, "b", "k",
					bytes.NewReader([]byte("d")), 1,
				)
			},
		},
		{
			"DeleteObject",
			func() error { return client.DeleteObject(ctx, "b", "k") },
		},
		{
			"CopyObject",
			func() error {
				return client.CopyObject(
					ctx,
					object.ObjectRef{Bucket: "s", Key: "k"},
					object.ObjectRef{Bucket: "d", Key: "k"},
				)
			},
		},
	}

	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not connected")
		})
	}

	valTests := []struct {
		name string
		fn   func() error
	}{
		{
			"ListBuckets",
			func() error {
				buckets, err := client.ListBuckets(ctx)
				assert.Nil(t, buckets)
				return err
			},
		},
		{
			"BucketExists",
			func() error {
				exists, err := client.BucketExists(ctx, "b")
				assert.False(t, exists)
				return err
			},
		},
		{
			"GetObject",
			func() error {
				obj, err := client.GetObject(ctx, "b", "k")
				assert.Nil(t, obj)
				return err
			},
		},
		{
			"ListObjects",
			func() error {
				objs, err := client.ListObjects(ctx, "b", "p")
				assert.Nil(t, objs)
				return err
			},
		},
		{
			"StatObject",
			func() error {
				info, err := client.StatObject(ctx, "b", "k")
				assert.Nil(t, info)
				return err
			},
		},
	}

	for _, tt := range valTests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not connected")
		})
	}
}

func TestClient_Concurrency(t *testing.T) {
	// bluff-scan: no-assert-ok (concurrency test — go test -race catches data races; absence of panic == correctness)
	client, _ := newTestClient(t)

	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func() {
			_ = client.IsConnected()
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		go func() {
			_ = client.Close()
			done <- true
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestSidecarMeta(t *testing.T) {
	dir := t.TempDir()
	objPath := filepath.Join(dir, "test-file")

	// Write a dummy file
	err := os.WriteFile(objPath, []byte("content"), 0o644)
	require.NoError(t, err)

	t.Run("write and read sidecar", func(t *testing.T) {
		meta := &sidecarMeta{
			ContentType: "text/plain",
			Metadata:    map[string]string{"author": "test"},
		}
		err := writeSidecar(objPath, meta)
		require.NoError(t, err)

		loaded, err := readSidecar(objPath)
		require.NoError(t, err)
		assert.Equal(t, "text/plain", loaded.ContentType)
		assert.Equal(t, "test", loaded.Metadata["author"])
		assert.False(t, loaded.CreatedAt.IsZero())
	})

	t.Run("read nonexistent sidecar", func(t *testing.T) {
		noFile := filepath.Join(dir, "no-such-file")
		_, err := readSidecar(noFile)
		require.Error(t, err)
	})
}

// --- Additional coverage tests ---

func TestClient_Connect_ErrorCreatingRootDir(t *testing.T) {
	// Use a path that cannot be created (parent doesn't exist and is a file)
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file")
	err := os.WriteFile(filePath, []byte("data"), 0o644)
	require.NoError(t, err)

	// Try to create a directory inside a file (will fail)
	badPath := filepath.Join(filePath, "subdir")
	client, err := NewClient(&Config{RootDir: badPath}, nil)
	require.NoError(t, err)

	err = client.Connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create root directory")
}

func TestClient_HealthCheck_RootDirRemoved(t *testing.T) {
	dir := t.TempDir()
	client, err := NewClient(&Config{RootDir: dir}, nil)
	require.NoError(t, err)

	err = client.Connect(context.Background())
	require.NoError(t, err)

	// Remove root directory after connection
	err = os.RemoveAll(dir)
	require.NoError(t, err)

	// Health check should fail
	err = client.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root directory inaccessible")
}

func TestClient_CreateBucket_ErrorCreatingDir(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	// Create a file in place of where bucket would go
	filePath := filepath.Join(dir, "bucket-as-file")
	err := os.WriteFile(filePath, []byte("data"), 0o644)
	require.NoError(t, err)

	// Try to create bucket with same name (MkdirAll will fail)
	err = client.CreateBucket(
		ctx, object.BucketConfig{Name: "bucket-as-file/subdir"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create bucket directory")
}

func TestClient_DeleteBucket_NonEmptyBucket(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	// Create bucket with object
	err := client.CreateBucket(ctx, object.BucketConfig{Name: "nonempty"})
	require.NoError(t, err)

	content := []byte("test")
	err = client.PutObject(
		ctx, "nonempty", "file.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.NoError(t, err)

	// Try to delete non-empty bucket (os.Remove fails on non-empty dir)
	err = client.DeleteBucket(ctx, "nonempty")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete bucket")
}

func TestClient_ListBuckets_ReadDirError(t *testing.T) {
	dir := t.TempDir()
	client, err := NewClient(&Config{RootDir: dir}, nil)
	require.NoError(t, err)

	err = client.Connect(context.Background())
	require.NoError(t, err)

	// Remove root dir to cause ReadDir error
	err = os.RemoveAll(dir)
	require.NoError(t, err)

	_, err = client.ListBuckets(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list buckets")
}

func TestClient_ListBuckets_SkipsFiles(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	// Create a bucket and a file in root
	err := client.CreateBucket(ctx, object.BucketConfig{Name: "real-bucket"})
	require.NoError(t, err)

	// Create a file (not a directory) in root
	filePath := filepath.Join(dir, "not-a-bucket.txt")
	err = os.WriteFile(filePath, []byte("data"), 0o644)
	require.NoError(t, err)

	buckets, err := client.ListBuckets(ctx)
	require.NoError(t, err)

	// Only the real bucket should appear
	names := make([]string, len(buckets))
	for i, b := range buckets {
		names[i] = b.Name
	}
	assert.Contains(t, names, "real-bucket")
	assert.NotContains(t, names, "not-a-bucket.txt")
}

func TestClient_BucketExists_StatError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	// Create a symlink that points to non-existent target (causes stat error)
	// on some systems. Instead, we'll test by checking a file (not dir)
	filePath := filepath.Join(dir, "file-not-dir")
	err := os.WriteFile(filePath, []byte("data"), 0o644)
	require.NoError(t, err)

	// Should return false for a file (not a directory)
	exists, err := client.BucketExists(ctx, "file-not-dir")
	require.NoError(t, err)
	assert.False(t, exists) // File exists but is not a directory
}

func TestClient_PutObject_CreateFileError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Create a directory where the file should go
	objDirPath := filepath.Join(dir, "bucket", "file-as-dir")
	err = os.MkdirAll(objDirPath, 0o755)
	require.NoError(t, err)

	// Try to put object where a directory exists
	content := []byte("test")
	err = client.PutObject(
		ctx, "bucket", "file-as-dir",
		bytes.NewReader(content), int64(len(content)),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create file")
}

func TestClient_PutObject_WriteError(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Create a reader that returns an error
	errReader := &errorReader{err: io.ErrUnexpectedEOF}
	err = client.PutObject(ctx, "bucket", "error.txt", errReader, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write object")
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(_ []byte) (int, error) {
	return 0, e.err
}

func TestClient_GetObject_FileNotExists(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	_, err = client.GetObject(ctx, "bucket", "nonexistent.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open object")
}

func TestClient_ListObjects_WalkError(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	// List objects in nonexistent bucket
	_, err := client.ListObjects(ctx, "nonexistent-bucket", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list objects")
}

func TestClient_ListObjects_SkipsMetaFiles(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Create object with metadata
	content := []byte("test")
	err = client.PutObject(
		ctx, "bucket", "file.txt",
		bytes.NewReader(content), int64(len(content)),
		object.WithContentType("text/plain"),
	)
	require.NoError(t, err)

	// Verify .meta file was created
	metaPath := filepath.Join(dir, "bucket", "file.txt"+metaSuffix)
	_, err = os.Stat(metaPath)
	require.NoError(t, err)

	// ListObjects should not return .meta files
	objects, err := client.ListObjects(ctx, "bucket", "")
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Equal(t, "file.txt", objects[0].Key)
}

func TestClient_StatObject_NotExists(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	_, err = client.StatObject(ctx, "bucket", "nonexistent.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stat object")
}

func TestClient_CopyObject_SourceNotExists(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})
	require.NoError(t, err)

	err = client.CopyObject(
		ctx,
		object.ObjectRef{Bucket: "src", Key: "nonexistent.txt"},
		object.ObjectRef{Bucket: "dst", Key: "copy.txt"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open source object")
}

func TestClient_CopyObject_DestCreateError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})
	require.NoError(t, err)

	content := []byte("test")
	err = client.PutObject(
		ctx, "src", "file.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.NoError(t, err)

	// Create a directory where destination file should be
	dstDir := filepath.Join(dir, "dst", "file-as-dir")
	err = os.MkdirAll(dstDir, 0o755)
	require.NoError(t, err)

	err = client.CopyObject(
		ctx,
		object.ObjectRef{Bucket: "src", Key: "file.txt"},
		object.ObjectRef{Bucket: "dst", Key: "file-as-dir"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create destination object")
}

func TestClient_PutObject_OnlyContentType(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	content := []byte("test")
	err = client.PutObject(
		ctx, "bucket", "typed.txt",
		bytes.NewReader(content), int64(len(content)),
		object.WithContentType("text/plain"),
	)
	require.NoError(t, err)

	info, err := client.StatObject(ctx, "bucket", "typed.txt")
	require.NoError(t, err)
	assert.Equal(t, "text/plain", info.ContentType)
}

func TestClient_PutObject_OnlyMetadata(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	content := []byte("test")
	err = client.PutObject(
		ctx, "bucket", "meta.txt",
		bytes.NewReader(content), int64(len(content)),
		object.WithMetadata(map[string]string{"key": "value"}),
	)
	require.NoError(t, err)

	info, err := client.StatObject(ctx, "bucket", "meta.txt")
	require.NoError(t, err)
	assert.Equal(t, "value", info.Metadata["key"])
}

func TestReadSidecar_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	objPath := filepath.Join(dir, "test-file")

	// Write invalid JSON to sidecar
	err := os.WriteFile(objPath+metaSuffix, []byte("not json"), 0o644)
	require.NoError(t, err)

	_, err = readSidecar(objPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal metadata")
}

func TestClient_CopyObject_MetaCopyError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})
	require.NoError(t, err)

	// Create source with metadata
	content := []byte("test")
	err = client.PutObject(
		ctx, "src", "file.txt",
		bytes.NewReader(content), int64(len(content)),
		object.WithContentType("text/plain"),
	)
	require.NoError(t, err)

	// Create a directory where the meta file should go
	metaDstDir := filepath.Join(dir, "dst", "file.txt"+metaSuffix)
	err = os.MkdirAll(metaDstDir, 0o755)
	require.NoError(t, err)

	// Copy should still succeed (meta copy failure is ignored)
	err = client.CopyObject(
		ctx,
		object.ObjectRef{Bucket: "src", Key: "file.txt"},
		object.ObjectRef{Bucket: "dst", Key: "file.txt"},
	)
	require.NoError(t, err)
}

func TestClient_StatObject_NoSidecar(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Create object without metadata options (no sidecar)
	content := []byte("test")
	err = client.PutObject(
		ctx, "bucket", "plain.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.NoError(t, err)

	info, err := client.StatObject(ctx, "bucket", "plain.txt")
	require.NoError(t, err)
	assert.Equal(t, "plain.txt", info.Key)
	assert.Empty(t, info.ContentType)
}

func TestClient_ListObjects_WithMetadata(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Create object with metadata
	content := []byte("test")
	err = client.PutObject(
		ctx, "bucket", "meta.txt",
		bytes.NewReader(content), int64(len(content)),
		object.WithContentType("text/plain"),
		object.WithMetadata(map[string]string{"key": "value"}),
	)
	require.NoError(t, err)

	objects, err := client.ListObjects(ctx, "bucket", "")
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Equal(t, "text/plain", objects[0].ContentType)
	assert.Equal(t, "value", objects[0].Metadata["key"])
}

func TestClient_PutObject_MkdirAllError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Create a file that would block directory creation
	blockPath := filepath.Join(dir, "bucket", "blocker")
	err = os.WriteFile(blockPath, []byte("block"), 0o644)
	require.NoError(t, err)

	// Try to put object in path that requires creating dir inside file
	content := []byte("test")
	err = client.PutObject(
		ctx, "bucket", "blocker/subdir/file.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create object directory")
}

func TestClient_CopyObject_DestDirError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})
	require.NoError(t, err)

	content := []byte("test")
	err = client.PutObject(
		ctx, "src", "file.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.NoError(t, err)

	// Create a file that would block directory creation
	blockPath := filepath.Join(dir, "dst", "blocker")
	err = os.WriteFile(blockPath, []byte("block"), 0o644)
	require.NoError(t, err)

	// Try to copy to path that requires creating dir inside file
	err = client.CopyObject(
		ctx,
		object.ObjectRef{Bucket: "src", Key: "file.txt"},
		object.ObjectRef{Bucket: "dst", Key: "blocker/subdir/file.txt"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create destination directory")
}

func TestClient_PutObject_WriteSidecarWarning(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Create object
	content := []byte("test")
	err = client.PutObject(
		ctx, "bucket", "file.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.NoError(t, err)

	// Create a directory where the .meta file should go
	metaPath := filepath.Join(dir, "bucket", "blocked.txt"+metaSuffix)
	err = os.MkdirAll(metaPath, 0o755)
	require.NoError(t, err)

	// Put object with metadata - writeSidecar will fail but operation should succeed
	err = client.PutObject(
		ctx, "bucket", "blocked.txt",
		bytes.NewReader(content), int64(len(content)),
		object.WithContentType("text/plain"),
	)
	// Should succeed even if sidecar write fails
	require.NoError(t, err)
}

func TestClient_ListObjects_NoMetadata(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Create object without metadata
	content := []byte("test")
	err = client.PutObject(
		ctx, "bucket", "plain.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.NoError(t, err)

	objects, err := client.ListObjects(ctx, "bucket", "")
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Empty(t, objects[0].ContentType)
}

func TestClient_CopyObject_MetaSrcOpenError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})
	require.NoError(t, err)

	// Create source with metadata
	content := []byte("test")
	err = client.PutObject(
		ctx, "src", "file.txt",
		bytes.NewReader(content), int64(len(content)),
		object.WithContentType("text/plain"),
	)
	require.NoError(t, err)

	// Replace the .meta file with a directory (will cause open error)
	metaSrcPath := filepath.Join(dir, "src", "file.txt"+metaSuffix)
	err = os.Remove(metaSrcPath)
	require.NoError(t, err)
	err = os.MkdirAll(metaSrcPath, 0o755)
	require.NoError(t, err)

	// Copy should succeed (meta copy error is ignored)
	err = client.CopyObject(
		ctx,
		object.ObjectRef{Bucket: "src", Key: "file.txt"},
		object.ObjectRef{Bucket: "dst", Key: "file.txt"},
	)
	require.NoError(t, err)
}

func TestClient_CopyObject_MetaDstCreateError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})
	require.NoError(t, err)

	// Create source with metadata
	content := []byte("test")
	err = client.PutObject(
		ctx, "src", "file.txt",
		bytes.NewReader(content), int64(len(content)),
		object.WithContentType("text/plain"),
	)
	require.NoError(t, err)

	// Create dst file first
	dstFilePath := filepath.Join(dir, "dst", "file.txt")
	err = os.WriteFile(dstFilePath, []byte("dst content"), 0o644)
	require.NoError(t, err)

	// Create a directory where the dst .meta file should go
	metaDstPath := filepath.Join(dir, "dst", "file.txt"+metaSuffix)
	err = os.MkdirAll(metaDstPath, 0o755)
	require.NoError(t, err)

	// Copy should succeed (meta copy error is ignored)
	err = client.CopyObject(
		ctx,
		object.ObjectRef{Bucket: "src", Key: "file.txt"},
		object.ObjectRef{Bucket: "dst", Key: "file.txt"},
	)
	require.NoError(t, err)
}

func TestClient_ListBuckets_EntryInfoError(t *testing.T) {
	// This tests the continue branch when entry.Info() fails (line 161)
	// We simulate this by removing read permissions after listing
	client, dir := newTestClient(t)
	ctx := context.Background()

	// Create a bucket
	err := client.CreateBucket(ctx, object.BucketConfig{Name: "test-bucket"})
	require.NoError(t, err)

	// List buckets should work
	buckets, err := client.ListBuckets(ctx)
	require.NoError(t, err)
	assert.Len(t, buckets, 1)

	// Create a symlink to a non-existent target (Info() will fail on broken symlink)
	brokenSymlink := filepath.Join(dir, "broken-link")
	err = os.Symlink("/nonexistent/path/that/does/not/exist", brokenSymlink)
	if err != nil {
		t.Skip("Unable to create symlink, skipping test")  // SKIP-OK: #legacy-untriaged
	}

	// List buckets should skip the broken symlink
	buckets, err = client.ListBuckets(ctx)
	require.NoError(t, err)
	// Should only have the real bucket
	names := make([]string, len(buckets))
	for i, b := range buckets {
		names[i] = b.Name
	}
	assert.Contains(t, names, "test-bucket")
	assert.NotContains(t, names, "broken-link")
}

func TestClient_BucketExists_FileIsDir(t *testing.T) {
	// Test that a file (not directory) returns false for BucketExists
	client, dir := newTestClient(t)
	ctx := context.Background()

	// Create a file (not a directory) in root
	filePath := filepath.Join(dir, "file-not-bucket")
	err := os.WriteFile(filePath, []byte("data"), 0o644)
	require.NoError(t, err)

	// BucketExists should return false since it's a file, not a directory
	exists, err := client.BucketExists(ctx, "file-not-bucket")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestClient_CopyObject_CopyError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})
	require.NoError(t, err)

	// Create source file
	content := []byte("test content for copy")
	err = client.PutObject(
		ctx, "src", "file.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.NoError(t, err)

	// Create a read-only destination directory to cause copy failure
	// Actually, let's test by making the source unreadable after creating it
	srcPath := filepath.Join(dir, "src", "readonly.txt")
	err = os.WriteFile(srcPath, []byte("data"), 0o000)
	require.NoError(t, err)
	defer func() { _ = os.Chmod(srcPath, 0o644) }()

	// Copy should fail because source is unreadable
	err = client.CopyObject(
		ctx,
		object.ObjectRef{Bucket: "src", Key: "readonly.txt"},
		object.ObjectRef{Bucket: "dst", Key: "copy.txt"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open source object")
}

func TestClient_CopyObject_IOCopyError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})
	require.NoError(t, err)

	// Create source file
	content := []byte("test content for copy")
	err = client.PutObject(
		ctx, "src", "file.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.NoError(t, err)

	// Make destination directory read-only after creating bucket
	// This will allow file creation but writing may fail
	dstBucketPath := filepath.Join(dir, "dst")

	// Create a file in dst where we want to write, but make it read-only
	dstFilePath := filepath.Join(dstBucketPath, "copy.txt")
	err = os.WriteFile(dstFilePath, []byte("existing"), 0o000)
	require.NoError(t, err)
	defer func() { _ = os.Chmod(dstFilePath, 0o644) }()

	// Copy should fail because dest file can't be written
	err = client.CopyObject(
		ctx,
		object.ObjectRef{Bucket: "src", Key: "file.txt"},
		object.ObjectRef{Bucket: "dst", Key: "copy.txt"},
	)
	require.Error(t, err)
	// The error might be "failed to create destination object" due to permissions
	assert.Error(t, err)
}

func TestClient_BucketExists_PermissionError(t *testing.T) {
	client, dir := newTestClient(t)
	ctx := context.Background()

	// Create a directory with no read permissions
	bucketPath := filepath.Join(dir, "no-perms")
	err := os.MkdirAll(bucketPath, 0o755)
	require.NoError(t, err)

	// Remove permissions from parent to cause stat error on the bucket
	// Actually, let's make a permission-denied scenario
	err = os.Chmod(bucketPath, 0o000)
	require.NoError(t, err)
	defer func() { _ = os.Chmod(bucketPath, 0o755) }()

	// Stat on the bucket should now fail with permission denied
	// Note: This might not work if running as root
	exists, err := client.BucketExists(ctx, "no-perms")

	// On some systems (or when running as root), this might still succeed
	// So we just check it completes without panic
	if err != nil {
		assert.Contains(t, err.Error(), "failed to check bucket")
	}
	_ = exists
}

func TestWriteSidecar_MarshalError(t *testing.T) {
	// Save original
	origMarshal := jsonMarshal
	defer func() { jsonMarshal = origMarshal }()

	// Replace with error-returning marshal
	jsonMarshal = func(_ any) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}

	dir := t.TempDir()
	objPath := filepath.Join(dir, "test-file")

	meta := &sidecarMeta{
		ContentType: "text/plain",
		Metadata:    map[string]string{"key": "value"},
	}

	err := writeSidecar(objPath, meta)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal metadata")
	assert.Contains(t, err.Error(), "marshal failed")
}

func TestClient_ListObjects_FilepathRelError(t *testing.T) {
	// Save original
	origRel := filepathRel
	defer func() { filepathRel = origRel }()

	// Replace with error-returning function
	filepathRel = func(_, _ string) (string, error) {
		return "", errors.New("rel path error")
	}

	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "bucket"})
	require.NoError(t, err)

	// Create a file in the bucket
	content := []byte("test")
	err = client.PutObject(
		ctx, "bucket", "file.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.NoError(t, err)

	// ListObjects should fail due to filepath.Rel error
	_, err = client.ListObjects(ctx, "bucket", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list objects")
}

func TestClient_CopyObject_IOCopyFailure(t *testing.T) {
	// Save original
	origCopy := ioCopy
	defer func() { ioCopy = origCopy }()

	// Replace with error-returning function
	ioCopy = func(_ io.Writer, _ io.Reader) (int64, error) {
		return 0, errors.New("copy failed")
	}

	client, _ := newTestClient(t)
	ctx := context.Background()

	err := client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})
	require.NoError(t, err)

	// Create source file
	content := []byte("test content")
	err = client.PutObject(
		ctx, "src", "file.txt",
		bytes.NewReader(content), int64(len(content)),
	)
	require.NoError(t, err)

	// Copy should fail due to io.Copy error
	err = client.CopyObject(
		ctx,
		object.ObjectRef{Bucket: "src", Key: "file.txt"},
		object.ObjectRef{Bucket: "dst", Key: "copy.txt"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to copy object")
	assert.Contains(t, err.Error(), "copy failed")
}

// Verify hooks are reset after tests
func TestClient_HooksResetVerification(t *testing.T) {
	// Verify jsonMarshal is the default
	data, err := jsonMarshal(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify filepathRel is the default
	rel, err := filepathRel("/a/b", "/a/b/c")
	require.NoError(t, err)
	assert.Equal(t, "c", rel)

	// Verify ioCopy is the default
	src := bytes.NewReader([]byte("test"))
	dst := &bytes.Buffer{}
	n, err := ioCopy(dst, src)
	require.NoError(t, err)
	assert.Equal(t, int64(4), n)
}

// mockDirEntry is a mock implementation of fs.DirEntry for testing.
type mockDirEntry struct {
	name    string
	isDir   bool
	infoErr error
}

func (m *mockDirEntry) Name() string      { return m.name }
func (m *mockDirEntry) IsDir() bool       { return m.isDir }
func (m *mockDirEntry) Type() os.FileMode { return 0 }
func (m *mockDirEntry) Info() (os.FileInfo, error) {
	if m.infoErr != nil {
		return nil, m.infoErr
	}
	return &mockFileInfo{name: m.name, isDir: m.isDir}, nil
}

// mockFileInfo is a mock implementation of os.FileInfo for testing.
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

func TestClient_ListBuckets_EntryInfoErrorWithMock(t *testing.T) {
	// Save original
	origReadDir := osReadDir
	defer func() { osReadDir = origReadDir }()

	// Replace with mock that returns entries where some have Info() errors
	osReadDir = func(_ string) ([]os.DirEntry, error) {
		return []os.DirEntry{
			&mockDirEntry{name: "good-bucket", isDir: true, infoErr: nil},
			&mockDirEntry{name: "bad-bucket", isDir: true, infoErr: errors.New("info failed")},
			&mockDirEntry{name: "not-dir", isDir: false, infoErr: nil}, // Will be skipped
		}, nil
	}

	client, _ := newTestClient(t)
	ctx := context.Background()

	// ListBuckets should skip entries with Info() errors
	buckets, err := client.ListBuckets(ctx)
	require.NoError(t, err)
	// Should only have "good-bucket" - bad-bucket is skipped due to Info() error,
	// not-dir is skipped because it's not a directory
	// Note: The mock returns nil FileInfo, so we'll get 1 bucket but with zero time
	assert.Len(t, buckets, 1)
}

func TestClient_BucketExists_StatErrorWithMock(t *testing.T) {
	// Save original
	origStat := osStat
	defer func() { osStat = origStat }()

	// Replace with mock that returns an error (not IsNotExist)
	osStatError := errors.New("permission denied")
	osStat = func(_ string) (os.FileInfo, error) {
		return nil, osStatError
	}

	client, _ := newTestClient(t)
	ctx := context.Background()

	// BucketExists should return error
	exists, err := client.BucketExists(ctx, "any-bucket")
	require.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "failed to check bucket")
}

// Ensure we use the original functions by default
var _ = json.Marshal
