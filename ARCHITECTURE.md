# Architecture -- Storage

## Purpose

Generic, reusable Go module for object storage operations. Provides a unified interface for S3-compatible storage (MinIO, AWS S3), local filesystem storage with sidecar metadata files, and cloud provider credential management (AWS, GCP, Azure).

## Structure

```
pkg/
  object/     Core interfaces: ObjectStore, BucketManager, types, functional options (PutOption)
  s3/         S3-compatible storage client (MinIO/AWS S3) with presigned URLs and lifecycle rules
  local/      Local filesystem storage with sidecar .meta JSON metadata files
  provider/   Cloud provider credential management: AWS, GCP, Azure abstractions
```

## Key Components

- **`object.ObjectStore`** -- Interface: Connect, Close, PutObject, GetObject, DeleteObject, ListObjects, StatObject, CopyObject, HealthCheck
- **`object.BucketManager`** -- Interface: CreateBucket, DeleteBucket, ListBuckets, BucketExists
- **`object.PutOption`** -- Functional options: WithContentType, WithMetadata for extensible upload configuration
- **`s3.Client`** -- MinIO/AWS S3 implementation with presigned URL generation and bucket lifecycle rules
- **`local.Store`** -- Filesystem-backed storage with atomic writes and `.meta` JSON sidecar files for metadata
- **`provider.CloudProvider`** -- Interface: Name, Credentials, HealthCheck for multi-cloud support

## Data Flow

```
s3.Client.PutObject(ctx, bucket, key, reader, size, opts...)
    |
    apply PutOption functions -> minio.PutObject() -> S3-compatible storage

local.Store.PutObject(ctx, bucket, key, reader, size, opts...)
    |
    create directory structure -> write file atomically -> write .meta JSON sidecar

provider.CloudProvider.Credentials() -> AWS/GCP/Azure credential chain
```

## Dependencies

- `github.com/minio/minio-go/v7` -- S3-compatible client
- `github.com/sirupsen/logrus` -- Structured logging
- `github.com/stretchr/testify` -- Test assertions

## Testing Strategy

Table-driven tests with `testify` and race detection. Local storage tests use temporary directories. S3 integration tests require a running MinIO instance. Tests cover object CRUD, bucket management, presigned URL generation, metadata sidecar files, and health check operations.
