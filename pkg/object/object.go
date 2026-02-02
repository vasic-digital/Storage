package object

import (
	"context"
	"io"
	"time"
)

// ObjectStore defines the interface for object storage operations.
type ObjectStore interface {
	// Connect establishes a connection to the object store.
	Connect(ctx context.Context) error

	// Close closes the connection to the object store.
	Close() error

	// PutObject uploads an object to the specified bucket.
	PutObject(
		ctx context.Context,
		bucket string,
		key string,
		reader io.Reader,
		size int64,
		opts ...PutOption,
	) error

	// GetObject retrieves an object from the specified bucket.
	GetObject(ctx context.Context, bucket string, key string) (io.ReadCloser, error)

	// DeleteObject removes an object from the specified bucket.
	DeleteObject(ctx context.Context, bucket string, key string) error

	// ListObjects lists objects in a bucket matching the given prefix.
	ListObjects(ctx context.Context, bucket string, prefix string) ([]ObjectInfo, error)

	// StatObject returns metadata about an object without downloading it.
	StatObject(ctx context.Context, bucket string, key string) (*ObjectInfo, error)

	// CopyObject copies an object from source to destination.
	CopyObject(ctx context.Context, src ObjectRef, dst ObjectRef) error

	// HealthCheck verifies connectivity to the object store.
	HealthCheck(ctx context.Context) error
}

// BucketManager defines the interface for bucket management operations.
type BucketManager interface {
	// CreateBucket creates a new bucket with the given configuration.
	CreateBucket(ctx context.Context, config BucketConfig) error

	// DeleteBucket removes a bucket by name.
	DeleteBucket(ctx context.Context, name string) error

	// ListBuckets returns all available buckets.
	ListBuckets(ctx context.Context) ([]BucketInfo, error)

	// BucketExists checks whether a bucket exists.
	BucketExists(ctx context.Context, name string) (bool, error)
}

// ObjectRef represents a reference to an object in a bucket.
type ObjectRef struct {
	Bucket string
	Key    string
}

// ObjectInfo holds metadata about a stored object.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
	ETag         string
	Metadata     map[string]string
}

// BucketInfo holds metadata about a bucket.
type BucketInfo struct {
	Name         string
	CreationDate time.Time
}

// BucketConfig holds configuration for creating a bucket.
type BucketConfig struct {
	Name          string
	Versioning    bool
	RetentionDays int
	ObjectLocking bool
}

// Config holds common configuration for object store connections.
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

// putOptions holds the resolved options for a put operation.
type putOptions struct {
	ContentType string
	Metadata    map[string]string
}

// PutOption is a functional option for PutObject operations.
type PutOption func(*putOptions)

// WithContentType sets the content type for the uploaded object.
func WithContentType(contentType string) PutOption {
	return func(o *putOptions) {
		o.ContentType = contentType
	}
}

// WithMetadata sets custom metadata on the uploaded object.
func WithMetadata(metadata map[string]string) PutOption {
	return func(o *putOptions) {
		o.Metadata = metadata
	}
}

// ResolvePutOptions applies all functional options and returns the result.
func ResolvePutOptions(opts ...PutOption) putOptions {
	var o putOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
