# Storage Module - Architecture

## Design Philosophy

The `digital.vasic.storage` module follows three core principles:

1. **Interface-first design**: All storage operations are defined through small, focused interfaces in the `object` package. Backend implementations are interchangeable.
2. **Zero coupling between backends**: The S3 and local filesystem backends share no code beyond the core interfaces. Each is self-contained.
3. **Separate credential management**: Cloud provider credentials are managed independently from storage operations, allowing flexible composition.

## Package Architecture

```
digital.vasic.storage/
  pkg/
    object/     Core interfaces, shared types, functional options
    s3/         S3-compatible storage implementation
    local/      Local filesystem storage implementation
    provider/   Cloud provider credential management
```

### Dependency Flow

```
           pkg/object (zero external deps)
           /          \
      pkg/s3        pkg/local
      (minio-go)    (os, filepath)

      pkg/provider (independent, zero internal deps)
```

The `object` package sits at the top of the dependency hierarchy. It depends only on the Go standard library (`context`, `io`, `time`). The `s3` and `local` packages import `object` for interface compliance and type reuse, but never import each other. The `provider` package is fully independent and does not depend on any other internal package.

## Design Patterns

### Adapter Pattern

Both `s3.Client` and `local.Client` act as adapters, translating the generic `ObjectStore` and `BucketManager` interfaces into backend-specific operations.

- **s3.Client** adapts `minio-go/v7` to `object.ObjectStore`
- **local.Client** adapts `os` file operations to `object.ObjectStore`

Each adapter translates:
- `PutObject` -> MinIO `PutObject` / `os.Create` + `io.Copy`
- `GetObject` -> MinIO `GetObject` / `os.Open`
- `ListObjects` -> MinIO `ListObjects` channel / `filepath.Walk`
- `CreateBucket` -> MinIO `MakeBucket` / `os.MkdirAll`

### Strategy Pattern

The `ObjectStore` interface enables the Strategy pattern. Callers program against the interface and can swap backends at runtime:

```go
var store object.ObjectStore

if useS3 {
    store, _ = s3.NewClient(s3Config, logger)
} else {
    store, _ = local.NewClient(localConfig, logger)
}

store.Connect(ctx)
store.PutObject(ctx, bucket, key, reader, size)
```

This is particularly useful for:
- Using local storage in tests and S3 in production
- Migrating between storage backends without changing application code
- Running in environments without network access

### Factory Pattern

Constructor functions serve as factories with built-in validation:

- `s3.NewClient(config, logger)` -- validates config, returns wired client
- `local.NewClient(config, logger)` -- validates root directory requirement
- `provider.NewAWSProvider(accessKey, secretKey, region, config)` -- validates required fields

Passing `nil` for optional parameters triggers sensible defaults:
- `s3.NewClient(nil, nil)` uses `DefaultConfig()` and a fresh `logrus.Logger`
- `provider.NewAWSProvider(..., nil)` uses `DefaultProviderConfig()`

### Builder Pattern

The S3 package uses the builder pattern for complex configuration objects. Methods return the receiver pointer for fluent chaining:

```go
bucket := s3.DefaultBucketConfig("data").
    WithRetention(90).
    WithVersioning().
    WithObjectLocking()

rule := s3.DefaultLifecycleRule("cleanup", 30).
    WithPrefix("temp/").
    WithNoncurrentExpiry(7)
```

Builder methods are defined on pointer receivers and always return `*T` for chaining.

### Functional Options Pattern

The `object` package uses functional options for extensible upload configuration without breaking the `PutObject` signature:

```go
type PutOption func(*putOptions)

func WithContentType(ct string) PutOption { ... }
func WithMetadata(m map[string]string) PutOption { ... }
```

Options are resolved by `ResolvePutOptions`, which applies them in order (last write wins). This pattern allows adding new options (e.g., `WithExpiration`, `WithEncryption`) without changing the interface.

### Interface Segregation

Storage operations are split into two interfaces:

- **ObjectStore**: Object CRUD (Connect, Close, PutObject, GetObject, DeleteObject, ListObjects, StatObject, CopyObject, HealthCheck)
- **BucketManager**: Bucket lifecycle (CreateBucket, DeleteBucket, ListBuckets, BucketExists)

Both `s3.Client` and `local.Client` implement both interfaces, but callers can accept only what they need:

```go
func upload(store object.ObjectStore, ...) { ... }
func setupBuckets(mgr object.BucketManager) { ... }
```

## Concurrency Design

### Mutex Strategy

Both clients use `sync.RWMutex` to protect the `connected` flag and underlying client references:

| Operation | Lock Type | Reason |
|-----------|-----------|--------|
| `Connect` | Write (`mu.Lock`) | Modifies connection state |
| `Close` | Write (`mu.Lock`) | Modifies connection state |
| All others | Read (`mu.RLock`) | Reads connection state only |

This allows full concurrency for storage operations while serializing connection lifecycle events. The S3 client's underlying `minio.Client` is itself goroutine-safe.

### Connection Guard

Every operation checks the `connected` flag before proceeding:

```go
func (c *Client) PutObject(...) error {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if !c.connected {
        return fmt.Errorf("not connected to S3")
    }
    // ... actual operation
}
```

This prevents nil pointer dereferences on the underlying client and provides clear error messages.

## Local Storage Design

### Directory Mapping

The local backend maps storage concepts to filesystem structures:

| Storage Concept | Filesystem Equivalent |
|----------------|----------------------|
| Bucket | Subdirectory under `rootDir` |
| Object key | File path relative to bucket directory |
| Object data | File contents |
| Object metadata | Sidecar `.meta` JSON file |

### Sidecar Metadata

Since filesystems do not natively support arbitrary key-value metadata, the local backend uses sidecar files:

```
rootDir/
  my-bucket/
    path/
      to/
        file.txt       # Object content
        file.txt.meta  # JSON metadata sidecar
```

The sidecar JSON schema:

```go
type sidecarMeta struct {
    ContentType string            `json:"content_type,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    CreatedAt   time.Time         `json:"created_at"`
}
```

Sidecar files are:
- Created only when `WithContentType` or `WithMetadata` options are used
- Loaded automatically by `StatObject` and `ListObjects`
- Copied alongside objects during `CopyObject`
- Deleted alongside objects during `DeleteObject`
- Filtered out from `ListObjects` results (any file ending in `.meta`)

### Path Normalization

Object keys use forward slashes regardless of platform. The local backend uses `filepath.ToSlash` when computing relative paths from `filepath.Walk`, ensuring consistent key formatting across operating systems.

## S3 Client Design

### MinIO SDK

The S3 backend uses `github.com/minio/minio-go/v7`, which provides:
- S3-compatible API for AWS S3, MinIO, and other S3-compatible services
- Static V4 credential authentication
- Multipart upload with configurable part size
- Presigned URL generation
- Bucket versioning and lifecycle management

### Config Validation

The `Config.Validate()` method enforces invariants at construction time rather than at runtime, following the "fail fast" principle. Required fields, positive durations, and minimum part sizes are all validated before a `Client` is created.

### Lifecycle Management

The S3 client supports bucket lifecycle rules through the MinIO SDK's lifecycle package. Rules can be:
- Added or updated by ID (`SetLifecycleRule`)
- Removed by ID (`RemoveLifecycleRule`)
- Applied automatically during `CreateBucket` when `RetentionDays > 0`

The `SetLifecycleRule` method merges with existing rules rather than replacing them.

## Provider Design

### Interface

The `CloudProvider` interface is intentionally minimal:

```go
type CloudProvider interface {
    Name() string
    Credentials() map[string]string
    HealthCheck(ctx context.Context) error
}
```

This serves as a credential abstraction layer, not a full cloud SDK wrapper. Consumers use `Credentials()` to extract configuration values for initializing cloud-specific clients.

### Provider Implementations

Each provider follows the same structure:

1. **Struct** with credential fields (exported for JSON/YAML serialization) and a private config
2. **Constructor** (`New*Provider`) with required-field validation
3. **Interface methods** (Name, Credentials, HealthCheck)
4. **Builder methods** (`With*`) for optional fields, returning `*T` for chaining
5. **Compile-time check**: `var _ CloudProvider = (*Provider)(nil)`

### Default Location Handling

GCP has a special case: if `location` is empty, it defaults to `"us-central1"`. This is handled in the constructor, not in a `DefaultConfig` function, because location is passed as a direct parameter rather than through a config struct.

## Error Handling Strategy

All errors follow a consistent wrapping pattern:

- **Operation errors**: `fmt.Errorf("failed to <verb> <noun>: %w", err)`
- **Connection errors**: `fmt.Errorf("not connected to <backend>")`
- **Validation errors**: `fmt.Errorf("<field> is required")`

The `%w` verb is used consistently to preserve the error chain for `errors.Is` and `errors.As` inspection.

## Testing Architecture

Tests are co-located with source files and follow these conventions:

- **Table-driven**: All test functions use `[]struct` test tables
- **Assertions**: `testify/assert` for soft assertions, `testify/require` for hard stops
- **Helper functions**: `newTestClient(t)` in local tests creates a temporary directory client
- **Guard tests**: Dedicated test functions verify all operations fail with `"not connected"` when the client is not connected
- **Concurrency tests**: Goroutine-based tests verify thread safety of `IsConnected` and `Close`
- **Interface compliance**: Compile-time checks (`var _ Interface = (*Struct)(nil)`) in production code, plus explicit interface cast tests
