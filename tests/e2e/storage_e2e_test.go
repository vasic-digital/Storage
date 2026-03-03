package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.storage/pkg/local"
	"digital.vasic.storage/pkg/object"
	"digital.vasic.storage/pkg/provider"
)

func TestEndToEnd_FullStorageWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Phase 1: Create bucket
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "e2e-bucket"})
	require.NoError(t, err)

	// Phase 2: Upload multiple objects
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("document-%d.txt", i)
		content := []byte(fmt.Sprintf("Document content #%d", i))
		err = client.PutObject(ctx, "e2e-bucket", key,
			bytes.NewReader(content), int64(len(content)),
			object.WithContentType("text/plain"),
			object.WithMetadata(map[string]string{
				"index": fmt.Sprintf("%d", i),
			}),
		)
		require.NoError(t, err)
	}

	// Phase 3: List and verify
	objects, err := client.ListObjects(ctx, "e2e-bucket", "document-")
	require.NoError(t, err)
	assert.Len(t, objects, 10)

	// Phase 4: Read each object and verify content
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("document-%d.txt", i)
		reader, err := client.GetObject(ctx, "e2e-bucket", key)
		require.NoError(t, err)

		data, err := io.ReadAll(reader)
		_ = reader.Close()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("Document content #%d", i), string(data))
	}

	// Phase 5: Copy and verify
	err = client.CopyObject(ctx,
		object.ObjectRef{Bucket: "e2e-bucket", Key: "document-0.txt"},
		object.ObjectRef{Bucket: "e2e-bucket", Key: "document-copy.txt"},
	)
	require.NoError(t, err)

	info, err := client.StatObject(ctx, "e2e-bucket", "document-copy.txt")
	require.NoError(t, err)
	assert.True(t, info.Size > 0)

	// Phase 6: Delete half the objects
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("document-%d.txt", i)
		err = client.DeleteObject(ctx, "e2e-bucket", key)
		require.NoError(t, err)
	}

	// Phase 7: Verify remaining
	objects, err = client.ListObjects(ctx, "e2e-bucket", "document-")
	require.NoError(t, err)
	assert.Len(t, objects, 6) // 5 remaining + 1 copy
}

func TestEndToEnd_MultipleTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	err = client.CreateBucket(ctx, object.BucketConfig{Name: "media"})
	require.NoError(t, err)

	// Upload different content types
	uploads := []struct {
		key         string
		contentType string
		data        []byte
	}{
		{"image.png", "image/png", []byte{0x89, 0x50, 0x4E, 0x47}},
		{"data.json", "application/json", []byte(`{"key": "value"}`)},
		{"style.css", "text/css", []byte("body { color: red; }")},
		{"script.js", "application/javascript", []byte("console.log('hi');")},
	}

	for _, u := range uploads {
		err = client.PutObject(ctx, "media", u.key,
			bytes.NewReader(u.data), int64(len(u.data)),
			object.WithContentType(u.contentType),
		)
		require.NoError(t, err)
	}

	// Verify content types via stat
	for _, u := range uploads {
		info, err := client.StatObject(ctx, "media", u.key)
		require.NoError(t, err)
		assert.Equal(t, u.contentType, info.ContentType,
			"content type mismatch for %s", u.key)
	}
}

func TestEndToEnd_ConnectionLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Not connected: operations should fail
	assert.False(t, client.IsConnected())
	_, err = client.ListBuckets(ctx)
	assert.Error(t, err)

	// Connect
	err = client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Operations should work
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "lifecycle"})
	assert.NoError(t, err)

	// Close
	err = client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())

	// Post-close: operations should fail
	_, err = client.ListBuckets(ctx)
	assert.Error(t, err)

	// Reconnect
	err = client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Bucket should still exist (filesystem persists)
	exists, err := client.BucketExists(ctx, "lifecycle")
	require.NoError(t, err)
	assert.True(t, exists)

	_ = client.Close()
}

func TestEndToEnd_AllProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	ctx := context.Background()

	providers := []struct {
		name string
		fn   func() (provider.CloudProvider, error)
	}{
		{"aws", func() (provider.CloudProvider, error) {
			return provider.NewAWSProvider("key", "secret", "us-east-1", nil)
		}},
		{"gcp", func() (provider.CloudProvider, error) {
			return provider.NewGCPProvider("project-1", "europe-west1", nil)
		}},
		{"azure", func() (provider.CloudProvider, error) {
			return provider.NewAzureProvider("sub-1", "tenant-1", nil)
		}},
	}

	for _, tc := range providers {
		t.Run(tc.name, func(t *testing.T) {
			p, err := tc.fn()
			require.NoError(t, err)
			assert.Equal(t, tc.name, p.Name())
			assert.NotEmpty(t, p.Credentials())
			assert.NoError(t, p.HealthCheck(ctx))
		})
	}
}

func TestEndToEnd_PutOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// Verify functional options resolve correctly
	opts := object.ResolvePutOptions(
		object.WithContentType("application/pdf"),
		object.WithMetadata(map[string]string{
			"title":  "Test Document",
			"author": "Integration Test",
		}),
	)
	assert.Equal(t, "application/pdf", opts.ContentType)
	assert.Equal(t, "Test Document", opts.Metadata["title"])
	assert.Equal(t, "Integration Test", opts.Metadata["author"])

	// Empty options
	emptyOpts := object.ResolvePutOptions()
	assert.Empty(t, emptyOpts.ContentType)
	assert.Nil(t, emptyOpts.Metadata)
}
