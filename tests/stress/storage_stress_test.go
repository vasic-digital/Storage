package stress

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.storage/pkg/local"
	"digital.vasic.storage/pkg/object"
	"digital.vasic.storage/pkg/provider"
)

func TestStress_ConcurrentObjectPut(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	err = client.CreateBucket(ctx, object.BucketConfig{Name: "stress"})
	require.NoError(t, err)

	const goroutines = 50
	const filesPerGoroutine = 20
	var wg sync.WaitGroup
	var errCount atomic.Int64

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < filesPerGoroutine; i++ {
				key := fmt.Sprintf("stress-%d/file-%d.txt", id, i)
				content := []byte(fmt.Sprintf("content-%d-%d", id, i))
				putErr := client.PutObject(ctx, "stress", key,
					bytes.NewReader(content), int64(len(content)))
				if putErr != nil {
					errCount.Add(1)
				}
			}
		}(g)
	}

	wg.Wait()
	assert.Equal(t, int64(0), errCount.Load(), "no errors during concurrent puts")

	// Verify total count
	objects, err := client.ListObjects(ctx, "stress", "")
	require.NoError(t, err)
	assert.Equal(t, goroutines*filesPerGoroutine, len(objects))
}

func TestStress_ConcurrentGetStat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	err = client.CreateBucket(ctx, object.BucketConfig{Name: "reads"})
	require.NoError(t, err)

	// Pre-populate
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("read-file-%d.txt", i)
		content := []byte(fmt.Sprintf("read-content-%d", i))
		err = client.PutObject(ctx, "reads", key,
			bytes.NewReader(content), int64(len(content)))
		require.NoError(t, err)
	}

	const goroutines = 80
	var wg sync.WaitGroup
	var errCount atomic.Int64

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				key := fmt.Sprintf("read-file-%d.txt", i%100)

				// Get
				reader, getErr := client.GetObject(ctx, "reads", key)
				if getErr != nil {
					errCount.Add(1)
					continue
				}
				_, _ = io.ReadAll(reader)
				_ = reader.Close()

				// Stat
				_, statErr := client.StatObject(ctx, "reads", key)
				if statErr != nil {
					errCount.Add(1)
				}
			}
		}(g)
	}

	wg.Wait()
	assert.Equal(t, int64(0), errCount.Load())
}

func TestStress_ConcurrentBucketOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	const goroutines = 50
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("bucket-%d", id)
			_ = client.CreateBucket(ctx, object.BucketConfig{Name: name})
			_, _ = client.BucketExists(ctx, name)
			_, _ = client.ListBuckets(ctx)
		}(g)
	}

	wg.Wait()

	buckets, err := client.ListBuckets(ctx)
	require.NoError(t, err)
	assert.Equal(t, goroutines, len(buckets))
}

func TestStress_ConcurrentCopyDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	err = client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	require.NoError(t, err)
	err = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})
	require.NoError(t, err)

	// Pre-populate source
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("src-file-%d.txt", i)
		content := []byte(fmt.Sprintf("content-%d", i))
		err = client.PutObject(ctx, "src", key,
			bytes.NewReader(content), int64(len(content)))
		require.NoError(t, err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	var errCount atomic.Int64

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			srcKey := fmt.Sprintf("src-file-%d.txt", id)
			dstKey := fmt.Sprintf("dst-file-%d.txt", id)

			copyErr := client.CopyObject(ctx,
				object.ObjectRef{Bucket: "src", Key: srcKey},
				object.ObjectRef{Bucket: "dst", Key: dstKey},
			)
			if copyErr != nil {
				errCount.Add(1)
			}
		}(g)
	}

	wg.Wait()
	assert.Equal(t, int64(0), errCount.Load())

	// Verify copies
	dstObjects, err := client.ListObjects(ctx, "dst", "")
	require.NoError(t, err)
	assert.Equal(t, goroutines, len(dstObjects))
}

func TestStress_ConcurrentProviderHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	aws, err := provider.NewAWSProvider("key", "secret", "us-east-1", nil)
	require.NoError(t, err)

	gcp, err := provider.NewGCPProvider("project", "us-central1", nil)
	require.NoError(t, err)

	azure, err := provider.NewAzureProvider("sub", "tenant", nil)
	require.NoError(t, err)

	providers := []provider.CloudProvider{aws, gcp, azure}

	const goroutines = 100
	var wg sync.WaitGroup
	var errCount atomic.Int64

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()
			p := providers[id%len(providers)]
			if err := p.HealthCheck(ctx); err != nil {
				errCount.Add(1)
			}
			_ = p.Name()
			_ = p.Credentials()
		}(g)
	}

	wg.Wait()
	assert.Equal(t, int64(0), errCount.Load())
}
