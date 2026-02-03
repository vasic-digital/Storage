# Storage Module - API Reference

Module: `digital.vasic.storage`

---

## Package `object`

**Import**: `digital.vasic.storage/pkg/object`

Core interfaces, shared types, and functional options for object storage operations.

### Interfaces

#### ObjectStore

```go
type ObjectStore interface {
    Connect(ctx context.Context) error
    Close() error
    PutObject(ctx context.Context, bucket string, key string,
        reader io.Reader, size int64, opts ...PutOption) error
    GetObject(ctx context.Context, bucket string, key string) (io.ReadCloser, error)
    DeleteObject(ctx context.Context, bucket string, key string) error
    ListObjects(ctx context.Context, bucket string, prefix string) ([]ObjectInfo, error)
    StatObject(ctx context.Context, bucket string, key string) (*ObjectInfo, error)
    CopyObject(ctx context.Context, src ObjectRef, dst ObjectRef) error
    HealthCheck(ctx context.Context) error
}
```

Defines the contract for object storage operations. Implemented by `s3.Client` and `local.Client`.

| Method | Description |
|--------|-------------|
| `Connect` | Establishes a connection to the object store. |
| `Close` | Closes the connection to the object store. |
| `PutObject` | Uploads an object to the specified bucket. Accepts functional options. |
| `GetObject` | Retrieves an object. Returns an `io.ReadCloser` that must be closed. |
| `DeleteObject` | Removes an object from the specified bucket. |
| `ListObjects` | Lists objects matching a prefix. Empty prefix lists all. |
| `StatObject` | Returns metadata about an object without downloading its body. |
| `CopyObject` | Copies an object from source to destination (cross-bucket supported). |
| `HealthCheck` | Verifies connectivity to the object store. |

#### BucketManager

```go
type BucketManager interface {
    CreateBucket(ctx context.Context, config BucketConfig) error
    DeleteBucket(ctx context.Context, name string) error
    ListBuckets(ctx context.Context) ([]BucketInfo, error)
    BucketExists(ctx context.Context, name string) (bool, error)
}
```

Defines the contract for bucket management operations. Implemented by `s3.Client` and `local.Client`.

| Method | Description |
|--------|-------------|
| `CreateBucket` | Creates a new bucket with the given configuration. |
| `DeleteBucket` | Removes a bucket by name. Bucket must be empty. |
| `ListBuckets` | Returns all available buckets. |
| `BucketExists` | Checks whether a bucket exists. |

### Types

#### ObjectRef

```go
type ObjectRef struct {
    Bucket string
    Key    string
}
```

A reference to an object in a bucket. Used by `CopyObject`.

#### ObjectInfo

```go
type ObjectInfo struct {
    Key          string
    Size         int64
    LastModified time.Time
    ContentType  string
    ETag         string
    Metadata     map[string]string
}
```

Metadata about a stored object. Returned by `ListObjects` and `StatObject`.

#### BucketInfo

```go
type BucketInfo struct {
    Name         string
    CreationDate time.Time
}
```

Metadata about a bucket. Returned by `ListBuckets`.

#### BucketConfig

```go
type BucketConfig struct {
    Name          string
    Versioning    bool
    RetentionDays int
    ObjectLocking bool
}
```

Configuration for creating a bucket. Used by `CreateBucket`.

| Field | Description |
|-------|-------------|
| `Name` | Bucket name (required). |
| `Versioning` | Enable object versioning. |
| `RetentionDays` | Auto-expire objects after N days (S3 lifecycle rule). |
| `ObjectLocking` | Enable object locking (WORM compliance). |

#### Config

```go
type Config struct {
    Endpoint       string        `json:"endpoint" yaml:"endpoint"`
    AccessKey      string        `json:"access_key" yaml:"access_key"`
    SecretKey      string        `json:"secret_key" yaml:"secret_key"`
    UseSSL         bool          `json:"use_ssl" yaml:"use_ssl"`
    Region         string        `json:"region" yaml:"region"`
    ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
    RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`
    MaxRetries     int           `json:"max_retries" yaml:"max_retries"`
    PartSize       int64         `json:"part_size" yaml:"part_size"`
}
```

Common configuration for object store connections. Supports JSON and YAML serialization.

### Functions

#### WithContentType

```go
func WithContentType(contentType string) PutOption
```

Returns a `PutOption` that sets the content type for the uploaded object.

#### WithMetadata

```go
func WithMetadata(metadata map[string]string) PutOption
```

Returns a `PutOption` that sets custom metadata on the uploaded object.

#### ResolvePutOptions

```go
func ResolvePutOptions(opts ...PutOption) putOptions
```

Applies all functional options in order and returns the resolved configuration. Options applied later override earlier ones (last-write-wins for `ContentType`; last-write replaces for `Metadata`).

### Type Aliases

#### PutOption

```go
type PutOption func(*putOptions)
```

Functional option type for configuring `PutObject` operations.

---

## Package `s3`

**Import**: `digital.vasic.storage/pkg/s3`

S3-compatible storage client supporting MinIO, AWS S3, and other S3-compatible services.

### Types

#### Client

```go
type Client struct { /* unexported fields */ }
```

Implements `object.ObjectStore` and `object.BucketManager` for S3-compatible storage.

##### NewClient

```go
func NewClient(config *Config, logger *logrus.Logger) (*Client, error)
```

Creates a new S3 client. If `config` is nil, `DefaultConfig()` is used. If `logger` is nil, a default logrus logger is created. Returns an error if config validation fails.

##### Client Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Connect` | `(ctx context.Context) error` | Connects to the S3 endpoint using MinIO SDK. Verifies connectivity by calling `ListBuckets`. |
| `Close` | `() error` | Disconnects and nils the internal MinIO client. |
| `IsConnected` | `() bool` | Returns connection status (thread-safe). |
| `HealthCheck` | `(ctx context.Context) error` | Calls `ListBuckets` to verify endpoint health. |
| `CreateBucket` | `(ctx context.Context, config object.BucketConfig) error` | Creates a bucket. Idempotent (no error if bucket exists). Enables versioning and lifecycle rules per config. |
| `DeleteBucket` | `(ctx context.Context, bucketName string) error` | Removes a bucket. |
| `ListBuckets` | `(ctx context.Context) ([]object.BucketInfo, error)` | Lists all buckets. |
| `BucketExists` | `(ctx context.Context, bucketName string) (bool, error)` | Checks bucket existence. |
| `PutObject` | `(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts ...object.PutOption) error` | Uploads an object with optional content type and metadata. Uses configured `PartSize` for multipart uploads. |
| `GetObject` | `(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error)` | Retrieves an object. |
| `DeleteObject` | `(ctx context.Context, bucketName, objectName string) error` | Removes an object. |
| `ListObjects` | `(ctx context.Context, bucketName, prefix string) ([]object.ObjectInfo, error)` | Lists objects recursively with optional prefix. |
| `StatObject` | `(ctx context.Context, bucketName, objectName string) (*object.ObjectInfo, error)` | Returns object metadata including user metadata. |
| `CopyObject` | `(ctx context.Context, src, dst object.ObjectRef) error` | Server-side copy between buckets. |
| `GetPresignedURL` | `(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error)` | Generates a presigned download URL. |
| `GetPresignedPutURL` | `(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error)` | Generates a presigned upload URL. |
| `SetLifecycleRule` | `(ctx context.Context, bucketName string, rule *LifecycleRule) error` | Sets or updates a lifecycle rule on a bucket. Merges with existing rules. |
| `RemoveLifecycleRule` | `(ctx context.Context, bucketName, ruleID string) error` | Removes a lifecycle rule by ID. |

#### Config

```go
type Config struct {
    Endpoint            string        `json:"endpoint" yaml:"endpoint"`
    AccessKey           string        `json:"access_key" yaml:"access_key"`
    SecretKey           string        `json:"secret_key" yaml:"secret_key"`
    UseSSL              bool          `json:"use_ssl" yaml:"use_ssl"`
    Region              string        `json:"region" yaml:"region"`
    ConnectTimeout      time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
    RequestTimeout      time.Duration `json:"request_timeout" yaml:"request_timeout"`
    MaxRetries          int           `json:"max_retries" yaml:"max_retries"`
    PartSize            int64         `json:"part_size" yaml:"part_size"`
    ConcurrentUploads   int           `json:"concurrent_uploads" yaml:"concurrent_uploads"`
    HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
}
```

S3-specific connection configuration.

##### DefaultConfig

```go
func DefaultConfig() *Config
```

Returns a config with sensible defaults for local MinIO:
- Endpoint: `localhost:9000`
- AccessKey: `minioadmin`
- SecretKey: `minioadmin123`
- UseSSL: `false`
- Region: `us-east-1`
- ConnectTimeout: `30s`
- RequestTimeout: `60s`
- MaxRetries: `3`
- PartSize: `16MB`
- ConcurrentUploads: `4`
- HealthCheckInterval: `30s`

##### Config.Validate

```go
func (c *Config) Validate() error
```

Validates all configuration fields. Returns an error describing the first invalid field.

#### BucketConfig

```go
type BucketConfig struct {
    Name          string `json:"name" yaml:"name"`
    RetentionDays int    `json:"retention_days" yaml:"retention_days"`
    Versioning    bool   `json:"versioning" yaml:"versioning"`
    ObjectLocking bool   `json:"object_locking" yaml:"object_locking"`
    Public        bool   `json:"public" yaml:"public"`
}
```

S3-specific bucket configuration with builder methods.

##### DefaultBucketConfig

```go
func DefaultBucketConfig(name string) *BucketConfig
```

Returns a `BucketConfig` with defaults (RetentionDays: -1, all booleans false).

##### BucketConfig Builder Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `WithRetention` | `(days int) *BucketConfig` | Sets retention days. Returns self for chaining. |
| `WithVersioning` | `() *BucketConfig` | Enables versioning. Returns self for chaining. |
| `WithObjectLocking` | `() *BucketConfig` | Enables object locking. Returns self for chaining. |
| `WithPublicAccess` | `() *BucketConfig` | Enables public read access. Returns self for chaining. |

#### LifecycleRule

```go
type LifecycleRule struct {
    ID                 string `json:"id" yaml:"id"`
    Prefix             string `json:"prefix" yaml:"prefix"`
    Enabled            bool   `json:"enabled" yaml:"enabled"`
    ExpirationDays     int    `json:"expiration_days" yaml:"expiration_days"`
    NoncurrentDays     int    `json:"noncurrent_days" yaml:"noncurrent_days"`
    DeleteMarkerExpiry bool   `json:"delete_marker_expiry" yaml:"delete_marker_expiry"`
}
```

Represents an object lifecycle rule.

##### DefaultLifecycleRule

```go
func DefaultLifecycleRule(id string, expirationDays int) *LifecycleRule
```

Returns a default lifecycle rule (enabled, no prefix filter, no noncurrent expiry).

##### LifecycleRule Builder Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `WithPrefix` | `(prefix string) *LifecycleRule` | Sets the prefix filter. Returns self for chaining. |
| `WithNoncurrentExpiry` | `(days int) *LifecycleRule` | Sets noncurrent version expiry days. Returns self for chaining. |

---

## Package `local`

**Import**: `digital.vasic.storage/pkg/local`

Local filesystem storage backend. Buckets map to directories, objects map to files.

### Types

#### Client

```go
type Client struct { /* unexported fields */ }
```

Implements `object.ObjectStore` and `object.BucketManager` using the local filesystem.

##### NewClient

```go
func NewClient(config *Config, logger *logrus.Logger) (*Client, error)
```

Creates a new local filesystem client. Returns an error if `config` is nil or `RootDir` is empty. If `logger` is nil, a default logrus logger is created.

##### Client Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Connect` | `(ctx context.Context) error` | Creates the root directory (if needed) via `os.MkdirAll` and marks client as connected. |
| `Close` | `() error` | Marks client as disconnected. Idempotent. |
| `IsConnected` | `() bool` | Returns connection status (thread-safe). |
| `HealthCheck` | `(ctx context.Context) error` | Verifies root directory is accessible via `os.Stat`. |
| `CreateBucket` | `(ctx context.Context, config object.BucketConfig) error` | Creates a subdirectory for the bucket. |
| `DeleteBucket` | `(ctx context.Context, name string) error` | Removes the bucket directory. Must be empty. |
| `ListBuckets` | `(ctx context.Context) ([]object.BucketInfo, error)` | Lists all subdirectories in root as buckets. |
| `BucketExists` | `(ctx context.Context, name string) (bool, error)` | Checks if the bucket subdirectory exists and is a directory. |
| `PutObject` | `(ctx context.Context, bucket, key string, reader io.Reader, size int64, opts ...object.PutOption) error` | Writes file to disk. Creates parent directories as needed. Writes `.meta` sidecar if options are provided. The `size` parameter is ignored. |
| `GetObject` | `(ctx context.Context, bucket, key string) (io.ReadCloser, error)` | Opens the file for reading. Returns `*os.File`. |
| `DeleteObject` | `(ctx context.Context, bucket, key string) error` | Removes the file and its `.meta` sidecar (if present). |
| `ListObjects` | `(ctx context.Context, bucket, prefix string) ([]object.ObjectInfo, error)` | Walks the bucket directory. Filters by prefix. Skips `.meta` files. Loads sidecar metadata. |
| `StatObject` | `(ctx context.Context, bucket, key string) (*object.ObjectInfo, error)` | Returns file metadata from `os.Stat` plus sidecar metadata. |
| `CopyObject` | `(ctx context.Context, src, dst object.ObjectRef) error` | Copies file contents and sidecar metadata to destination. |

#### Config

```go
type Config struct {
    RootDir string `json:"root_dir" yaml:"root_dir"`
}
```

Configuration for the local filesystem client.

| Field | Description |
|-------|-------------|
| `RootDir` | Root directory where bucket subdirectories are stored. Required. |

### Constants

```go
const metaSuffix = ".meta"
```

File suffix for sidecar metadata files (unexported, internal use).

---

## Package `provider`

**Import**: `digital.vasic.storage/pkg/provider`

Cloud provider credential management for AWS, GCP, and Azure.

### Interfaces

#### CloudProvider

```go
type CloudProvider interface {
    Name() string
    Credentials() map[string]string
    HealthCheck(ctx context.Context) error
}
```

Defines the contract for cloud provider credential and health management.

| Method | Description |
|--------|-------------|
| `Name` | Returns the provider name (e.g., `"aws"`, `"gcp"`, `"azure"`). |
| `Credentials` | Returns credentials as a key-value map. |
| `HealthCheck` | Verifies that credentials are configured and valid. |

### Types

#### ProviderConfig

```go
type ProviderConfig struct {
    Timeout time.Duration `json:"timeout" yaml:"timeout"`
}
```

Common configuration for cloud providers.

##### DefaultProviderConfig

```go
func DefaultProviderConfig() *ProviderConfig
```

Returns a config with default timeout of 30 seconds.

#### AWSProvider

```go
type AWSProvider struct {
    AccessKeyID     string `json:"access_key_id" yaml:"access_key_id"`
    SecretAccessKey  string `json:"secret_access_key" yaml:"secret_access_key"`
    Region          string `json:"region" yaml:"region"`
    SessionToken    string `json:"session_token,omitempty" yaml:"session_token"`
}
```

Manages AWS credentials. Implements `CloudProvider`.

##### NewAWSProvider

```go
func NewAWSProvider(
    accessKeyID string,
    secretAccessKey string,
    region string,
    config *ProviderConfig,
) (*AWSProvider, error)
```

Creates a new AWS provider. All three string parameters are required. Returns error if any is empty. Nil `config` defaults to `DefaultProviderConfig()`.

##### AWSProvider Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Name` | `() string` | Returns `"aws"`. |
| `Credentials` | `() map[string]string` | Returns map with keys: `access_key_id`, `secret_access_key`, `region`, and optionally `session_token`. |
| `HealthCheck` | `(ctx context.Context) error` | Verifies `AccessKeyID` and `SecretAccessKey` are non-empty. |
| `WithSessionToken` | `(token string) *AWSProvider` | Sets session token for temporary credentials. Returns self for chaining. |

#### GCPProvider

```go
type GCPProvider struct {
    ProjectID      string `json:"project_id" yaml:"project_id"`
    ServiceAccount string `json:"service_account,omitempty" yaml:"service_account"`
    Location       string `json:"location" yaml:"location"`
}
```

Manages GCP credentials. Implements `CloudProvider`.

##### NewGCPProvider

```go
func NewGCPProvider(
    projectID string,
    location string,
    config *ProviderConfig,
) (*GCPProvider, error)
```

Creates a new GCP provider. `projectID` is required. Empty `location` defaults to `"us-central1"`. Nil `config` defaults to `DefaultProviderConfig()`.

##### GCPProvider Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Name` | `() string` | Returns `"gcp"`. |
| `Credentials` | `() map[string]string` | Returns map with keys: `project_id`, `location`, and optionally `service_account`. |
| `HealthCheck` | `(ctx context.Context) error` | Verifies `ProjectID` is non-empty. |
| `WithServiceAccount` | `(sa string) *GCPProvider` | Sets the service account email. Returns self for chaining. |

#### AzureProvider

```go
type AzureProvider struct {
    SubscriptionID string `json:"subscription_id" yaml:"subscription_id"`
    TenantID       string `json:"tenant_id" yaml:"tenant_id"`
    ClientID       string `json:"client_id,omitempty" yaml:"client_id"`
    ClientSecret   string `json:"client_secret,omitempty" yaml:"client_secret"`
}
```

Manages Azure credentials. Implements `CloudProvider`.

##### NewAzureProvider

```go
func NewAzureProvider(
    subscriptionID string,
    tenantID string,
    config *ProviderConfig,
) (*AzureProvider, error)
```

Creates a new Azure provider. Both `subscriptionID` and `tenantID` are required. Nil `config` defaults to `DefaultProviderConfig()`.

##### AzureProvider Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Name` | `() string` | Returns `"azure"`. |
| `Credentials` | `() map[string]string` | Returns map with keys: `subscription_id`, `tenant_id`, and optionally `client_id`, `client_secret`. |
| `HealthCheck` | `(ctx context.Context) error` | Verifies `SubscriptionID` and `TenantID` are non-empty. |
| `WithClientCredentials` | `(clientID, clientSecret string) *AzureProvider` | Sets service principal credentials. Returns self for chaining. |

### Compile-Time Checks

```go
var (
    _ CloudProvider = (*AWSProvider)(nil)
    _ CloudProvider = (*GCPProvider)(nil)
    _ CloudProvider = (*AzureProvider)(nil)
)
```

All three providers are verified at compile time to satisfy the `CloudProvider` interface.
