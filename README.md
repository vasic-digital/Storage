# Storage

Generic, reusable Go module for object storage operations.

**Module**: `digital.vasic.storage` (Go 1.24+)

## Packages

- **pkg/object** -- Core interfaces (`ObjectStore`, `BucketManager`) and shared types
- **pkg/s3** -- S3-compatible storage client (MinIO, AWS S3) with presigned URLs and lifecycle rules
- **pkg/local** -- Local filesystem storage with sidecar `.meta` JSON metadata files
- **pkg/provider** -- Cloud provider credential management (AWS, GCP, Azure)

## Quick Start

```go
import (
    "digital.vasic.storage/pkg/s3"
    "digital.vasic.storage/pkg/object"
)

// Create S3 client
client, err := s3.NewClient(s3.DefaultConfig(), nil)
if err != nil {
    log.Fatal(err)
}

// Connect
if err := client.Connect(ctx); err != nil {
    log.Fatal(err)
}
defer client.Close()

// Upload object
err = client.PutObject(ctx, "my-bucket", "file.txt", reader, size,
    object.WithContentType("text/plain"),
    object.WithMetadata(map[string]string{"author": "test"}),
)
```

## Testing

```bash
go test ./... -count=1 -race
```

## License

Proprietary.
