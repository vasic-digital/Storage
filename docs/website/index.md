# digital.vasic.storage

A Go module for unified object storage operations across S3-compatible services (MinIO, AWS S3), local filesystem, and cloud provider credential management.

## Key Features

- **Unified Object Store Interface** -- `ObjectStore` and `BucketManager` interfaces for CRUD, copy, health check, and presigned URLs
- **S3 Client** -- Full MinIO/AWS S3 integration with bucket lifecycle rules, versioning, and presigned URL generation
- **Local Filesystem Client** -- Files as objects, directories as buckets, with `.meta` JSON sidecar metadata
- **Cloud Providers** -- AWS, GCP, and Azure credential management behind a `CloudProvider` interface
- **Storage Resolver** -- Strategy-pattern routing of asset paths to storage backends by prefix rules
- **Functional Options** -- Extensible `PutOption` for content type and metadata on uploads

## Installation

```bash
go get digital.vasic.storage
```

Requires Go 1.24+.

## Package Overview

| Package | Import Path | Purpose |
|---------|-------------|---------|
| `object` | `digital.vasic.storage/pkg/object` | Core interfaces (`ObjectStore`, `BucketManager`), types, and functional options |
| `s3` | `digital.vasic.storage/pkg/s3` | S3-compatible client via MinIO SDK, presigned URLs, lifecycle rules |
| `local` | `digital.vasic.storage/pkg/local` | Local filesystem storage with sidecar metadata |
| `provider` | `digital.vasic.storage/pkg/provider` | Cloud provider credential management (AWS, GCP, Azure) |
| `resolver` | `digital.vasic.storage/pkg/resolver` | Strategy-based routing of paths to backends |

## Quick Example

```go
package main

import (
    "context"
    "strings"

    "digital.vasic.storage/pkg/local"
    "digital.vasic.storage/pkg/object"
)

func main() {
    client, _ := local.NewClient(&local.Config{RootDir: "/tmp/storage"}, nil)
    ctx := context.Background()

    client.Connect(ctx)
    defer client.Close()

    client.CreateBucket(ctx, object.BucketConfig{Name: "uploads"})

    client.PutObject(ctx, "uploads", "readme.txt",
        strings.NewReader("Hello, Storage!"), 15,
        object.WithContentType("text/plain"),
    )

    reader, _ := client.GetObject(ctx, "uploads", "readme.txt")
    defer reader.Close()
}
```
