# Course: Object Storage Abstraction in Go

## Module Overview

This course covers the `digital.vasic.storage` module, which provides a unified object storage abstraction with S3 and local filesystem backends, cloud provider credential management, and functional options for extensible configuration. You will learn to design interface-segregated storage APIs, implement the Adapter pattern for multiple backends, and use the Builder pattern for S3 bucket configuration.

## Prerequisites

- Intermediate Go knowledge (interfaces, functional options, I/O)
- Basic understanding of S3-compatible object storage
- Familiarity with filesystem operations
- Go 1.24+ installed

## Lessons

| # | Title | Duration |
|---|-------|----------|
| 1 | Core Interfaces and Local Storage | 45 min |
| 2 | S3-Compatible Storage Backend | 50 min |
| 3 | Cloud Providers and Storage Routing | 40 min |

## Source Files

- `pkg/object/` -- Core interfaces (`ObjectStore`, `BucketManager`), shared types, functional options
- `pkg/s3/` -- S3-compatible storage (MinIO SDK) with lifecycle management
- `pkg/local/` -- Local filesystem storage with sidecar metadata
- `pkg/provider/` -- Cloud provider credential abstraction (AWS, GCP, Azure)
- `pkg/resolver/` -- Storage resolver utilities
