package s3

import (
	"fmt"
	"time"
)

// Config holds S3-compatible storage connection configuration.
type Config struct {
	// Connection settings
	Endpoint  string `json:"endpoint" yaml:"endpoint"`
	AccessKey string `json:"access_key" yaml:"access_key"`
	SecretKey string `json:"secret_key" yaml:"secret_key"`
	UseSSL    bool   `json:"use_ssl" yaml:"use_ssl"`
	Region    string `json:"region" yaml:"region"`

	// Connection options
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`
	MaxRetries     int           `json:"max_retries" yaml:"max_retries"`

	// Upload settings
	PartSize          int64 `json:"part_size" yaml:"part_size"`
	ConcurrentUploads int   `json:"concurrent_uploads" yaml:"concurrent_uploads"`

	// Health check
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
}

// DefaultConfig returns a Config with sensible defaults for local MinIO.
func DefaultConfig() *Config {
	return &Config{
		Endpoint:            "localhost:9000",
		AccessKey:           "minioadmin",
		SecretKey:           "minioadmin123",
		UseSSL:              false,
		Region:              "us-east-1",
		ConnectTimeout:      30 * time.Second,
		RequestTimeout:      60 * time.Second,
		MaxRetries:          3,
		PartSize:            16 * 1024 * 1024, // 16MB
		ConcurrentUploads:   4,
		HealthCheckInterval: 30 * time.Second,
	}
}

// Validate checks that all required fields are set and values are sensible.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if c.AccessKey == "" {
		return fmt.Errorf("access_key is required")
	}
	if c.SecretKey == "" {
		return fmt.Errorf("secret_key is required")
	}
	if c.ConnectTimeout <= 0 {
		return fmt.Errorf("connect_timeout must be positive")
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("request_timeout must be positive")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}
	if c.PartSize < 5*1024*1024 {
		return fmt.Errorf("part_size must be at least 5MB")
	}
	if c.ConcurrentUploads < 1 {
		return fmt.Errorf("concurrent_uploads must be at least 1")
	}
	return nil
}

// BucketConfig holds bucket-specific configuration for S3.
type BucketConfig struct {
	Name          string `json:"name" yaml:"name"`
	RetentionDays int    `json:"retention_days" yaml:"retention_days"`
	Versioning    bool   `json:"versioning" yaml:"versioning"`
	ObjectLocking bool   `json:"object_locking" yaml:"object_locking"`
	Public        bool   `json:"public" yaml:"public"`
}

// DefaultBucketConfig returns a BucketConfig with defaults.
func DefaultBucketConfig(name string) *BucketConfig {
	return &BucketConfig{
		Name:          name,
		RetentionDays: -1,
		Versioning:    false,
		ObjectLocking: false,
		Public:        false,
	}
}

// WithRetention sets retention days and returns the config for chaining.
func (bc *BucketConfig) WithRetention(days int) *BucketConfig {
	bc.RetentionDays = days
	return bc
}

// WithVersioning enables versioning and returns the config for chaining.
func (bc *BucketConfig) WithVersioning() *BucketConfig {
	bc.Versioning = true
	return bc
}

// WithObjectLocking enables object locking and returns the config for chaining.
func (bc *BucketConfig) WithObjectLocking() *BucketConfig {
	bc.ObjectLocking = true
	return bc
}

// WithPublicAccess enables public read access and returns the config for chaining.
func (bc *BucketConfig) WithPublicAccess() *BucketConfig {
	bc.Public = true
	return bc
}

// LifecycleRule represents an object lifecycle rule.
type LifecycleRule struct {
	ID                 string `json:"id" yaml:"id"`
	Prefix             string `json:"prefix" yaml:"prefix"`
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	ExpirationDays     int    `json:"expiration_days" yaml:"expiration_days"`
	NoncurrentDays     int    `json:"noncurrent_days" yaml:"noncurrent_days"`
	DeleteMarkerExpiry bool   `json:"delete_marker_expiry" yaml:"delete_marker_expiry"`
}

// DefaultLifecycleRule returns a default lifecycle rule.
func DefaultLifecycleRule(id string, expirationDays int) *LifecycleRule {
	return &LifecycleRule{
		ID:                 id,
		Prefix:             "",
		Enabled:            true,
		ExpirationDays:     expirationDays,
		NoncurrentDays:     0,
		DeleteMarkerExpiry: false,
	}
}

// WithPrefix sets the prefix filter and returns the rule for chaining.
func (lr *LifecycleRule) WithPrefix(prefix string) *LifecycleRule {
	lr.Prefix = prefix
	return lr
}

// WithNoncurrentExpiry sets noncurrent version expiry and returns the rule
// for chaining.
func (lr *LifecycleRule) WithNoncurrentExpiry(days int) *LifecycleRule {
	lr.NoncurrentDays = days
	return lr
}
