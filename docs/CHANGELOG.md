# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-01-15

### Added

- **pkg/object**: Core interfaces and shared types.
  - `ObjectStore` interface with Connect, Close, PutObject, GetObject, DeleteObject, ListObjects, StatObject, CopyObject, HealthCheck.
  - `BucketManager` interface with CreateBucket, DeleteBucket, ListBuckets, BucketExists.
  - Shared types: `ObjectRef`, `ObjectInfo`, `BucketInfo`, `BucketConfig`, `Config`.
  - Functional options pattern: `PutOption`, `WithContentType`, `WithMetadata`, `ResolvePutOptions`.

- **pkg/s3**: S3-compatible storage client.
  - Full `ObjectStore` and `BucketManager` implementation using `minio-go/v7`.
  - `Config` with `Validate()` and `DefaultConfig()` for local MinIO defaults.
  - `BucketConfig` builder: `DefaultBucketConfig`, `WithRetention`, `WithVersioning`, `WithObjectLocking`, `WithPublicAccess`.
  - `LifecycleRule` builder: `DefaultLifecycleRule`, `WithPrefix`, `WithNoncurrentExpiry`.
  - Presigned URL generation: `GetPresignedURL`, `GetPresignedPutURL`.
  - Lifecycle rule management: `SetLifecycleRule`, `RemoveLifecycleRule`.
  - Thread-safe with `sync.RWMutex`.
  - Compile-time interface compliance checks.

- **pkg/local**: Local filesystem storage client.
  - Full `ObjectStore` and `BucketManager` implementation using `os` and `filepath`.
  - Directory-as-bucket mapping with nested key support.
  - Sidecar `.meta` JSON files for content type and custom metadata.
  - Automatic sidecar creation, deletion, and copying.
  - Thread-safe with `sync.RWMutex`.
  - Compile-time interface compliance checks.

- **pkg/provider**: Cloud provider credential management.
  - `CloudProvider` interface: Name, Credentials, HealthCheck.
  - `AWSProvider` with `WithSessionToken` builder.
  - `GCPProvider` with `WithServiceAccount` builder (defaults location to `us-central1`).
  - `AzureProvider` with `WithClientCredentials` builder.
  - `ProviderConfig` with `DefaultProviderConfig`.
  - Compile-time interface compliance checks for all three providers.

- Comprehensive unit tests for all packages with table-driven tests, concurrency tests, and not-connected guard tests.
