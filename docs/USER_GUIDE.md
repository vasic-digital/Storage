# Storage Module - User Guide

## Introduction

`digital.vasic.storage` is a generic, reusable Go module for object storage operations. It provides a unified interface for multiple storage backends and cloud provider credential management.

**Supported backends:**
- **S3-compatible** (AWS S3, MinIO, DigitalOcean Spaces, Backblaze B2)
- **Local filesystem** (development, testing, edge deployments)

**Supported cloud providers:**
- AWS (IAM credentials, session tokens)
- GCP (project ID, service accounts)
- Azure (subscription/tenant, service principal)

## Installation

```bash
go get digital.vasic.storage@latest
```

Requires Go 1.24 or later.

## Quick Start

### S3/MinIO Storage

```go
package main

import (
    "bytes"
    "context"
    "log"

    "digital.vasic.storage/pkg/object"
    "digital.vasic.storage/pkg/s3"
)

func main() {
    ctx := context.Background()

    // Create client with default MinIO config
    client, err := s3.NewClient(s3.DefaultConfig(), nil)
    if err != nil {
        log.Fatal(err)
    }

    // Connect to the S3 endpoint
    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create a bucket
    err = client.CreateBucket(ctx, object.BucketConfig{
        Name:       "my-bucket",
        Versioning: true,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Upload an object
    data := []byte("Hello, Storage!")
    err = client.PutObject(
        ctx, "my-bucket", "greeting.txt",
        bytes.NewReader(data), int64(len(data)),
        object.WithContentType("text/plain"),
        object.WithMetadata(map[string]string{
            "author":  "example",
            "version": "1",
        }),
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Object uploaded successfully")
}
```

### Local Filesystem Storage

```go
package main

import (
    "bytes"
    "context"
    "io"
    "log"

    "digital.vasic.storage/pkg/local"
    "digital.vasic.storage/pkg/object"
)

func main() {
    ctx := context.Background()

    // Create local storage client
    client, err := local.NewClient(
        &local.Config{RootDir: "/var/data/storage"},
        nil, // uses default logrus logger
    )
    if err != nil {
        log.Fatal(err)
    }

    // Connect (creates root directory if needed)
    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create a bucket (creates a subdirectory)
    err = client.CreateBucket(ctx, object.BucketConfig{
        Name: "documents",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Upload a file
    content := []byte(`{"key": "value"}`)
    err = client.PutObject(
        ctx, "documents", "data.json",
        bytes.NewReader(content), int64(len(content)),
        object.WithContentType("application/json"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Read it back
    reader, err := client.GetObject(ctx, "documents", "data.json")
    if err != nil {
        log.Fatal(err)
    }
    defer reader.Close()

    got, _ := io.ReadAll(reader)
    log.Printf("Content: %s\n", got)
}
```

## Object Operations

### Uploading Objects

Use `PutObject` with functional options to control content type and metadata:

```go
err := client.PutObject(
    ctx, "bucket", "path/to/file.pdf",
    reader, fileSize,
    object.WithContentType("application/pdf"),
    object.WithMetadata(map[string]string{
        "uploaded_by": "user123",
        "category":    "reports",
    }),
)
```

The `size` parameter is required for S3 (used for multipart upload decisions) but ignored by the local backend.

### Downloading Objects

`GetObject` returns an `io.ReadCloser`. Always close it when done:

```go
reader, err := client.GetObject(ctx, "bucket", "path/to/file.pdf")
if err != nil {
    return err
}
defer reader.Close()

data, err := io.ReadAll(reader)
```

### Inspecting Objects

Use `StatObject` to get metadata without downloading the object body:

```go
info, err := client.StatObject(ctx, "bucket", "path/to/file.pdf")
if err != nil {
    return err
}

fmt.Printf("Key: %s\n", info.Key)
fmt.Printf("Size: %d bytes\n", info.Size)
fmt.Printf("Content-Type: %s\n", info.ContentType)
fmt.Printf("Last Modified: %s\n", info.LastModified)
fmt.Printf("ETag: %s\n", info.ETag)
fmt.Printf("Metadata: %v\n", info.Metadata)
```

### Listing Objects

List objects with an optional prefix filter:

```go
// List all objects in a bucket
objects, err := client.ListObjects(ctx, "bucket", "")

// List objects under a prefix
objects, err := client.ListObjects(ctx, "bucket", "reports/2025/")

for _, obj := range objects {
    fmt.Printf("%s (%d bytes)\n", obj.Key, obj.Size)
}
```

### Copying Objects

Copy objects within or across buckets:

```go
err := client.CopyObject(
    ctx,
    object.ObjectRef{Bucket: "source-bucket", Key: "original.txt"},
    object.ObjectRef{Bucket: "backup-bucket", Key: "copy.txt"},
)
```

The local backend also copies sidecar metadata files automatically.

### Deleting Objects

```go
err := client.DeleteObject(ctx, "bucket", "path/to/file.pdf")
```

The local backend also removes the associated `.meta` sidecar file.

## Bucket Management

### Creating Buckets

```go
err := client.CreateBucket(ctx, object.BucketConfig{
    Name:          "my-bucket",
    Versioning:    true,
    RetentionDays: 90,
    ObjectLocking: true,
})
```

The S3 client is idempotent -- creating an existing bucket returns no error.

### Listing Buckets

```go
buckets, err := client.ListBuckets(ctx)
for _, b := range buckets {
    fmt.Printf("%s (created: %s)\n", b.Name, b.CreationDate)
}
```

### Checking Bucket Existence

```go
exists, err := client.BucketExists(ctx, "my-bucket")
```

### Deleting Buckets

```go
err := client.DeleteBucket(ctx, "my-bucket")
```

Note: buckets must be empty before deletion.

## S3-Specific Features

### Custom Configuration

```go
config := &s3.Config{
    Endpoint:            "s3.amazonaws.com",
    AccessKey:           "AKIAIOSFODNN7EXAMPLE",
    SecretKey:           "wJalrXUtnFEMI/K7MDENG",
    UseSSL:              true,
    Region:              "us-west-2",
    ConnectTimeout:      30 * time.Second,
    RequestTimeout:      60 * time.Second,
    MaxRetries:          3,
    PartSize:            16 * 1024 * 1024, // 16MB
    ConcurrentUploads:   4,
    HealthCheckInterval: 30 * time.Second,
}

client, err := s3.NewClient(config, nil)
```

Config validation is automatic. The `Validate()` method enforces:
- Endpoint, AccessKey, SecretKey are non-empty
- ConnectTimeout and RequestTimeout are positive
- MaxRetries is non-negative
- PartSize is at least 5MB
- ConcurrentUploads is at least 1

### Presigned URLs

Generate temporary download/upload URLs for sharing:

```go
// Download URL (expires in 1 hour)
downloadURL, err := client.GetPresignedURL(
    ctx, "bucket", "file.pdf", time.Hour,
)

// Upload URL (expires in 15 minutes)
uploadURL, err := client.GetPresignedPutURL(
    ctx, "bucket", "uploads/new-file.pdf", 15*time.Minute,
)
```

### Bucket Configuration Builder

Use the fluent builder API for bucket configuration:

```go
bucketCfg := s3.DefaultBucketConfig("compliance-data").
    WithRetention(365).
    WithVersioning().
    WithObjectLocking().
    WithPublicAccess()
```

### Lifecycle Rules

Manage automatic object expiration:

```go
// Create a lifecycle rule
rule := s3.DefaultLifecycleRule("expire-logs", 30).
    WithPrefix("logs/").
    WithNoncurrentExpiry(7)

err := client.SetLifecycleRule(ctx, "my-bucket", rule)

// Remove a lifecycle rule
err = client.RemoveLifecycleRule(ctx, "my-bucket", "expire-logs")
```

Lifecycle rules support:
- Prefix-based filtering
- Expiration days for current versions
- Noncurrent version expiration
- Delete marker cleanup

## Cloud Provider Credentials

### AWS

```go
awsProvider, err := provider.NewAWSProvider(
    "AKIAIOSFODNN7EXAMPLE",
    "wJalrXUtnFEMI/K7MDENG",
    "us-east-1",
    nil, // uses default config (30s timeout)
)

// Optional: add session token for temporary credentials
awsProvider.WithSessionToken("FwoGZXIvYXdzEA...")

// Get credentials map
creds := awsProvider.Credentials()
// creds["access_key_id"], creds["secret_access_key"],
// creds["region"], creds["session_token"]

// Health check
err = awsProvider.HealthCheck(ctx)
```

### GCP

```go
gcpProvider, err := provider.NewGCPProvider(
    "my-gcp-project",
    "europe-west1",   // empty string defaults to "us-central1"
    nil,
)

// Optional: set service account
gcpProvider.WithServiceAccount("sa@project.iam.gserviceaccount.com")

creds := gcpProvider.Credentials()
// creds["project_id"], creds["location"], creds["service_account"]
```

### Azure

```go
azureProvider, err := provider.NewAzureProvider(
    "subscription-id-123",
    "tenant-id-456",
    nil,
)

// Optional: add service principal credentials
azureProvider.WithClientCredentials("client-id", "client-secret")

creds := azureProvider.Credentials()
// creds["subscription_id"], creds["tenant_id"],
// creds["client_id"], creds["client_secret"]
```

## Health Checks

All clients expose a `HealthCheck` method:

```go
if err := client.HealthCheck(ctx); err != nil {
    log.Printf("Storage unhealthy: %v", err)
}
```

- **S3 client**: Calls `ListBuckets` to verify endpoint connectivity.
- **Local client**: Verifies the root directory is accessible via `os.Stat`.
- **Cloud providers**: Verify that required credentials are configured.

## Connection Lifecycle

All storage clients follow the same lifecycle pattern:

```go
// 1. Create
client, err := s3.NewClient(config, logger)

// 2. Connect
err = client.Connect(ctx)

// 3. Check connection status
if client.IsConnected() { ... }

// 4. Perform operations
err = client.PutObject(...)

// 5. Close
err = client.Close()
```

Operations called before `Connect` or after `Close` return a `"not connected"` error.

## Error Handling

All errors are wrapped with descriptive context:

```go
reader, err := client.GetObject(ctx, "bucket", "missing-file.txt")
if err != nil {
    // err: "failed to open object: open /path/to/file: no such file or directory"
    // err: "failed to get object: The specified key does not exist."
}
```

Use `errors.Is` and `errors.As` to inspect underlying errors:

```go
if errors.Is(err, os.ErrNotExist) {
    // Handle missing file in local storage
}
```

## Concurrency

Both `s3.Client` and `local.Client` are safe for concurrent use. They use `sync.RWMutex` internally to serialize connection state changes while allowing concurrent read operations.

```go
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        key := fmt.Sprintf("file-%d.txt", id)
        data := []byte(fmt.Sprintf("content %d", id))
        _ = client.PutObject(
            ctx, "bucket", key,
            bytes.NewReader(data), int64(len(data)),
        )
    }(i)
}
wg.Wait()
```

## Local Storage Metadata

The local backend stores object metadata in sidecar `.meta` JSON files alongside the object data files:

```
rootDir/
  bucket-name/
    file.txt         # Object data
    file.txt.meta    # Sidecar metadata JSON
```

Sidecar metadata structure:

```json
{
    "content_type": "text/plain",
    "metadata": {
        "author": "test",
        "version": "1"
    },
    "created_at": "2025-01-15T10:00:00Z"
}
```

Sidecar files are automatically created when `WithContentType` or `WithMetadata` options are used, and are automatically deleted alongside the object and copied during `CopyObject`.

## Testing

Run all tests:

```bash
go test ./... -count=1 -race
```

Run unit tests only (no infrastructure required):

```bash
go test ./... -short
```

Run integration tests (requires a running MinIO instance):

```bash
go test -tags=integration ./...
```
