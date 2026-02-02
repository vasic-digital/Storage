# CLAUDE.md - Storage Module

## Overview

`digital.vasic.storage` is a generic, reusable Go module for object storage operations. It provides a unified interface for S3-compatible storage (MinIO, AWS S3), local filesystem storage, and cloud provider credential management.

**Module**: `digital.vasic.storage` (Go 1.24+)

## Build & Test

```bash
go build ./...
go test ./... -count=1 -race
go test ./... -short              # Unit tests only
go test -tags=integration ./...   # Integration tests (requires MinIO)
go test -bench=. ./tests/benchmark/
```

## Code Style

- Standard Go conventions, `gofmt` formatting
- Imports grouped: stdlib, third-party, internal (blank line separated)
- Line length <= 100 chars
- Naming: `camelCase` private, `PascalCase` exported, acronyms all-caps
- Errors: always check, wrap with `fmt.Errorf("...: %w", err)`
- Tests: table-driven, `testify`, naming `Test<Struct>_<Method>_<Scenario>`

## Package Structure

| Package | Purpose |
|---------|---------|
| `pkg/object` | Core interfaces: ObjectStore, BucketManager, types, functional options |
| `pkg/s3` | S3-compatible storage client (MinIO/AWS S3), presigned URLs, lifecycle |
| `pkg/local` | Local filesystem storage with sidecar `.meta` JSON metadata |
| `pkg/provider` | Cloud provider abstractions: AWS, GCP, Azure credential management |

## Key Interfaces

- `object.ObjectStore` -- Object CRUD: Connect, Close, PutObject, GetObject, DeleteObject, ListObjects, StatObject, CopyObject, HealthCheck
- `object.BucketManager` -- Bucket management: CreateBucket, DeleteBucket, ListBuckets, BucketExists
- `provider.CloudProvider` -- Cloud providers: Name, Credentials, HealthCheck

## Design Patterns

- **Interface Segregation**: ObjectStore and BucketManager are separate interfaces
- **Functional Options**: PutOption for extensible upload configuration
- **Strategy**: Multiple storage backends (S3, local) behind same interface
- **Builder**: Chained configuration (BucketConfig, LifecycleRule)

## Dependencies

- `github.com/minio/minio-go/v7` -- S3-compatible client
- `github.com/sirupsen/logrus` -- Structured logging
- `github.com/stretchr/testify` -- Testing assertions

## Commit Style

Conventional Commits: `feat(s3): add presigned URL generation`
