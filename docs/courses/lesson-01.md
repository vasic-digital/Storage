# Lesson 1: Core Interfaces and Local Storage

## Objectives

- Understand the `ObjectStore` and `BucketManager` interfaces
- Use functional options for upload configuration
- Work with the local filesystem client

## Concepts

### ObjectStore Interface

All storage backends implement:

```go
type ObjectStore interface {
    Connect(ctx context.Context) error
    Close() error
    PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, opts ...PutOption) error
    GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)
    DeleteObject(ctx context.Context, bucket, key string) error
    ListObjects(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error)
    StatObject(ctx context.Context, bucket, key string) (*ObjectInfo, error)
    CopyObject(ctx context.Context, src, dst ObjectRef) error
    HealthCheck(ctx context.Context) error
}
```

### BucketManager Interface

```go
type BucketManager interface {
    CreateBucket(ctx context.Context, config BucketConfig) error
    DeleteBucket(ctx context.Context, name string) error
    ListBuckets(ctx context.Context) ([]BucketInfo, error)
    BucketExists(ctx context.Context, name string) (bool, error)
}
```

### Functional Options

```go
client.PutObject(ctx, "bucket", "key", reader, size,
    object.WithContentType("application/json"),
    object.WithMetadata(map[string]string{"version": "1"}),
)
```

## Code Walkthrough

### Setting up the local client

```go
client, err := local.NewClient(&local.Config{
    RootDir: "/data/storage",
}, nil) // nil = default logrus logger

ctx := context.Background()
client.Connect(ctx)
defer client.Close()
```

`Connect` creates the root directory if it does not exist.

### Bucket operations

```go
client.CreateBucket(ctx, object.BucketConfig{Name: "media"})

exists, _ := client.BucketExists(ctx, "media") // true

buckets, _ := client.ListBuckets(ctx)
// [{Name:"media", CreationDate:...}]
```

### Object operations

```go
// Upload
client.PutObject(ctx, "media", "video.mp4", file, fileSize,
    object.WithContentType("video/mp4"))

// List
objects, _ := client.ListObjects(ctx, "media", "")

// Stat
info, _ := client.StatObject(ctx, "media", "video.mp4")
fmt.Println(info.Size, info.ContentType)

// Copy
client.CopyObject(ctx,
    object.ObjectRef{Bucket: "media", Key: "video.mp4"},
    object.ObjectRef{Bucket: "media", Key: "backup/video.mp4"},
)

// Delete
client.DeleteObject(ctx, "media", "video.mp4")
```

## Summary

The `object` package defines clean, separated interfaces for object and bucket operations. The `local` client maps these to the filesystem with sidecar metadata files, making it ideal for development and testing.
