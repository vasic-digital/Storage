# Getting Started

## Install

```bash
go get digital.vasic.storage
```

## Local Filesystem Storage

The `local` package maps buckets to directories and objects to files.

```go
import (
    "digital.vasic.storage/pkg/local"
    "digital.vasic.storage/pkg/object"
)

client, err := local.NewClient(&local.Config{RootDir: "/data/storage"}, nil)
ctx := context.Background()

client.Connect(ctx)
defer client.Close()
```

### Create a bucket

```go
client.CreateBucket(ctx, object.BucketConfig{Name: "images"})
```

### Upload an object

```go
file, _ := os.Open("photo.jpg")
defer file.Close()

info, _ := file.Stat()
client.PutObject(ctx, "images", "photo.jpg", file, info.Size(),
    object.WithContentType("image/jpeg"),
    object.WithMetadata(map[string]string{"author": "alice"}),
)
```

Metadata is stored in a `.meta` JSON sidecar file alongside the object.

### List, stat, copy, and delete

```go
objects, _ := client.ListObjects(ctx, "images", "")
stat, _ := client.StatObject(ctx, "images", "photo.jpg")

client.CopyObject(ctx,
    object.ObjectRef{Bucket: "images", Key: "photo.jpg"},
    object.ObjectRef{Bucket: "images", Key: "backup/photo.jpg"},
)

client.DeleteObject(ctx, "images", "photo.jpg")
```

## S3-Compatible Storage

The `s3` package uses the MinIO SDK for any S3-compatible endpoint.

```go
import "digital.vasic.storage/pkg/s3"

cfg := s3.DefaultConfig()              // localhost:9000, minioadmin
cfg.Endpoint = "s3.amazonaws.com"
cfg.AccessKey = os.Getenv("AWS_ACCESS_KEY")
cfg.SecretKey = os.Getenv("AWS_SECRET_KEY")
cfg.UseSSL = true

client, err := s3.NewClient(cfg, nil)
client.Connect(ctx)
```

### Presigned URLs

```go
url, _ := client.GetPresignedURL(ctx, "uploads", "report.pdf", 1*time.Hour)
putURL, _ := client.GetPresignedPutURL(ctx, "uploads", "report.pdf", 15*time.Minute)
```

### Lifecycle Rules

```go
rule := s3.DefaultLifecycleRule("expire-logs", 30).
    WithPrefix("logs/").
    WithNoncurrentExpiry(7)

client.SetLifecycleRule(ctx, "my-bucket", rule)
```

## Storage Resolver

Route asset paths to different backends based on prefix rules.

```go
import "digital.vasic.storage/pkg/resolver"

r := resolver.New()
r.RegisterBackend(localBackend)
r.RegisterBackend(s3Backend)
r.AddRule("images/", "local")
r.AddRule("archives/", "s3")
r.SetFallback("local")

reader, _ := r.Read(ctx, "images/photo.jpg") // routes to local
```
