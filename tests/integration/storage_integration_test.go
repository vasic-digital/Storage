package integration

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.storage/pkg/local"
	"digital.vasic.storage/pkg/object"
	"digital.vasic.storage/pkg/provider"
)

func TestLocalClient_BucketLifecycle_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Create buckets
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "test-bucket-1"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "test-bucket-2"})
	require.NoError(t, err)

	// List buckets
	buckets, err := client.ListBuckets(ctx)
	require.NoError(t, err)
	assert.Len(t, buckets, 2)

	// Check existence
	exists, err := client.BucketExists(ctx, "test-bucket-1")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = client.BucketExists(ctx, "non-existent")
	require.NoError(t, err)
	assert.False(t, exists)

	// Delete bucket
	err = client.DeleteBucket(ctx, "test-bucket-2")
	require.NoError(t, err)

	buckets, err = client.ListBuckets(ctx)
	require.NoError(t, err)
	assert.Len(t, buckets, 1)
	assert.Equal(t, "test-bucket-1", buckets[0].Name)
}

func TestLocalClient_ObjectCRUD_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	err = client.CreateBucket(ctx, object.BucketConfig{Name: "objects"})
	require.NoError(t, err)

	// Put object
	content := []byte("Hello, Storage Integration Test!")
	err = client.PutObject(ctx, "objects", "greeting.txt",
		bytes.NewReader(content), int64(len(content)),
		object.WithContentType("text/plain"),
		object.WithMetadata(map[string]string{"author": "test"}),
	)
	require.NoError(t, err)

	// Get object
	reader, err := client.GetObject(ctx, "objects", "greeting.txt")
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	// Stat object
	info, err := client.StatObject(ctx, "objects", "greeting.txt")
	require.NoError(t, err)
	assert.Equal(t, "greeting.txt", info.Key)
	assert.Equal(t, int64(len(content)), info.Size)
	assert.Equal(t, "text/plain", info.ContentType)
	assert.Equal(t, "test", info.Metadata["author"])

	// List objects
	objects, err := client.ListObjects(ctx, "objects", "")
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Equal(t, "greeting.txt", objects[0].Key)

	// Delete object
	err = client.DeleteObject(ctx, "objects", "greeting.txt")
	require.NoError(t, err)

	objects, err = client.ListObjects(ctx, "objects", "")
	require.NoError(t, err)
	assert.Len(t, objects, 0)
}

func TestLocalClient_CopyObject_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	err = client.CreateBucket(ctx, object.BucketConfig{Name: "src-bucket"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst-bucket"})
	require.NoError(t, err)

	content := []byte("copy me across buckets")
	err = client.PutObject(ctx, "src-bucket", "original.txt",
		bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	// Copy object
	err = client.CopyObject(ctx,
		object.ObjectRef{Bucket: "src-bucket", Key: "original.txt"},
		object.ObjectRef{Bucket: "dst-bucket", Key: "copied.txt"},
	)
	require.NoError(t, err)

	// Verify copy
	reader, err := client.GetObject(ctx, "dst-bucket", "copied.txt")
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestLocalClient_HealthCheck_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Health check before connect should fail
	err = client.HealthCheck(ctx)
	assert.Error(t, err)

	// Connect and health check should succeed
	err = client.Connect(ctx)
	require.NoError(t, err)

	err = client.HealthCheck(ctx)
	assert.NoError(t, err)

	// Close and health check should fail
	_ = client.Close()
	err = client.HealthCheck(ctx)
	assert.Error(t, err)
}

func TestLocalClient_NestedObjects_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	err = client.CreateBucket(ctx, object.BucketConfig{Name: "nested"})
	require.NoError(t, err)

	// Create nested objects
	paths := []string{
		"dir1/file1.txt",
		"dir1/dir2/file2.txt",
		"dir1/dir2/dir3/file3.txt",
	}

	for _, p := range paths {
		content := []byte("content for " + p)
		err = client.PutObject(ctx, "nested", p,
			bytes.NewReader(content), int64(len(content)))
		require.NoError(t, err)
	}

	// List with prefix
	objects, err := client.ListObjects(ctx, "nested", "dir1/dir2/")
	require.NoError(t, err)
	assert.Len(t, objects, 2, "should find 2 objects under dir1/dir2/")

	// Verify nested directory was created on disk
	nestedPath := filepath.Join(tmpDir, "nested", "dir1", "dir2", "dir3")
	_, err = os.Stat(nestedPath)
	assert.NoError(t, err, "nested directory should exist on disk")
}

func TestCloudProviders_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	ctx := context.Background()

	// AWS provider
	aws, err := provider.NewAWSProvider("AKID", "secret", "us-east-1", nil)
	require.NoError(t, err)
	assert.Equal(t, "aws", aws.Name())
	creds := aws.Credentials()
	assert.Equal(t, "AKID", creds["access_key_id"])
	assert.NoError(t, aws.HealthCheck(ctx))

	// GCP provider
	gcp, err := provider.NewGCPProvider("my-project", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "gcp", gcp.Name())
	assert.Equal(t, "us-central1", gcp.Credentials()["location"])
	assert.NoError(t, gcp.HealthCheck(ctx))

	// Azure provider
	azure, err := provider.NewAzureProvider("sub-123", "tenant-456", nil)
	require.NoError(t, err)
	assert.Equal(t, "azure", azure.Name())
	assert.NoError(t, azure.HealthCheck(ctx))
}
