package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"digital.vasic.storage/pkg/local"
	"digital.vasic.storage/pkg/object"
	"digital.vasic.storage/pkg/provider"
)

func BenchmarkLocalClient_PutObject(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	tmpDir := b.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	_ = client.Connect(ctx)
	defer func() { _ = client.Close() }()

	_ = client.CreateBucket(ctx, object.BucketConfig{Name: "bench"})
	content := bytes.Repeat([]byte("benchmark-data"), 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-file-%d.txt", i)
		_ = client.PutObject(ctx, "bench", key,
			bytes.NewReader(content), int64(len(content)))
	}
}

func BenchmarkLocalClient_GetObject(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	tmpDir := b.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	_ = client.Connect(ctx)
	defer func() { _ = client.Close() }()

	_ = client.CreateBucket(ctx, object.BucketConfig{Name: "bench"})

	// Pre-populate
	content := bytes.Repeat([]byte("benchmark-data"), 100)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("bench-file-%d.txt", i)
		_ = client.PutObject(ctx, "bench", key,
			bytes.NewReader(content), int64(len(content)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-file-%d.txt", i%1000)
		reader, err := client.GetObject(ctx, "bench", key)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.ReadAll(reader)
		_ = reader.Close()
	}
}

func BenchmarkLocalClient_StatObject(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	tmpDir := b.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	_ = client.Connect(ctx)
	defer func() { _ = client.Close() }()

	_ = client.CreateBucket(ctx, object.BucketConfig{Name: "bench"})

	content := []byte("stat-bench-content")
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("stat-file-%d.txt", i)
		_ = client.PutObject(ctx, "bench", key,
			bytes.NewReader(content), int64(len(content)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("stat-file-%d.txt", i%500)
		_, _ = client.StatObject(ctx, "bench", key)
	}
}

func BenchmarkLocalClient_ListObjects(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	tmpDir := b.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	_ = client.Connect(ctx)
	defer func() { _ = client.Close() }()

	_ = client.CreateBucket(ctx, object.BucketConfig{Name: "bench"})

	content := []byte("list-bench-content")
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("list-file-%d.txt", i)
		_ = client.PutObject(ctx, "bench", key,
			bytes.NewReader(content), int64(len(content)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.ListObjects(ctx, "bench", "list-")
	}
}

func BenchmarkLocalClient_CopyObject(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	tmpDir := b.TempDir()
	client, err := local.NewClient(&local.Config{RootDir: tmpDir}, nil)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	_ = client.Connect(ctx)
	defer func() { _ = client.Close() }()

	_ = client.CreateBucket(ctx, object.BucketConfig{Name: "src"})
	_ = client.CreateBucket(ctx, object.BucketConfig{Name: "dst"})

	content := bytes.Repeat([]byte("copy-bench-data"), 100)
	_ = client.PutObject(ctx, "src", "source.txt",
		bytes.NewReader(content), int64(len(content)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dstKey := fmt.Sprintf("copy-%d.txt", i)
		_ = client.CopyObject(ctx,
			object.ObjectRef{Bucket: "src", Key: "source.txt"},
			object.ObjectRef{Bucket: "dst", Key: dstKey},
		)
	}
}

func BenchmarkAWSProvider_Credentials(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	p, _ := provider.NewAWSProvider("key", "secret", "us-east-1", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Credentials()
	}
}

func BenchmarkPutOptions_Resolve(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	opts := []object.PutOption{
		object.WithContentType("application/json"),
		object.WithMetadata(map[string]string{
			"key1": "value1",
			"key2": "value2",
		}),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = object.ResolvePutOptions(opts...)
	}
}
