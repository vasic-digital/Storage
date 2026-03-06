# Lesson 2: S3-Compatible Storage

## Objectives

- Connect to MinIO or AWS S3
- Generate presigned URLs for uploads and downloads
- Manage bucket lifecycle rules

## Concepts

### S3 Configuration

```go
cfg := &s3.Config{
    Endpoint:            "localhost:9000",
    AccessKey:           "minioadmin",
    SecretKey:           "minioadmin123",
    UseSSL:              false,
    Region:              "us-east-1",
    ConnectTimeout:      30 * time.Second,
    RequestTimeout:      60 * time.Second,
    MaxRetries:          3,
    PartSize:            16 * 1024 * 1024, // 16 MB
    ConcurrentUploads:   4,
    HealthCheckInterval: 30 * time.Second,
}
```

`Config.Validate()` checks all required fields and value constraints.

### Presigned URLs

Generate time-limited URLs for direct client access without exposing credentials:

- `GetPresignedURL` -- download URL with expiry
- `GetPresignedPutURL` -- upload URL with expiry

### Lifecycle Rules

Automate object expiration and cleanup:

```go
rule := s3.DefaultLifecycleRule("cleanup-temp", 7).
    WithPrefix("tmp/").
    WithNoncurrentExpiry(3)
```

## Code Walkthrough

### Connecting

```go
client, err := s3.NewClient(cfg, nil)
ctx := context.Background()
err = client.Connect(ctx) // verifies connectivity by listing buckets
defer client.Close()
```

### Bucket creation with versioning

```go
client.CreateBucket(ctx, object.BucketConfig{
    Name:       "documents",
    Versioning: true,
})
```

The S3 client enables versioning and sets lifecycle rules as part of bucket creation when configured.

### Presigned URLs

```go
// Download URL valid for 1 hour
downloadURL, _ := client.GetPresignedURL(ctx, "documents", "report.pdf", time.Hour)

// Upload URL valid for 15 minutes
uploadURL, _ := client.GetPresignedPutURL(ctx, "documents", "upload.pdf", 15*time.Minute)
```

### Managing lifecycle rules

```go
// Set a rule
rule := s3.DefaultLifecycleRule("expire-logs", 30)
client.SetLifecycleRule(ctx, "my-bucket", rule)

// Remove a rule
client.RemoveLifecycleRule(ctx, "my-bucket", "expire-logs")
```

Rules are merged with existing rules by ID. If a rule with the same ID exists, it is replaced.

## Summary

The `s3` package provides a production-ready S3 client with presigned URLs, lifecycle management, and full compatibility with both MinIO and AWS S3.
