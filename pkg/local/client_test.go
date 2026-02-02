package local

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

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
