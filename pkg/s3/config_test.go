package s3

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "localhost:9000", config.Endpoint)
	assert.Equal(t, "minioadmin", config.AccessKey)
	assert.Equal(t, "minioadmin123", config.SecretKey)
	assert.False(t, config.UseSSL)
	assert.Equal(t, "us-east-1", config.Region)
	assert.Equal(t, 30*time.Second, config.ConnectTimeout)
	assert.Equal(t, 60*time.Second, config.RequestTimeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, int64(16*1024*1024), config.PartSize)
	assert.Equal(t, 4, config.ConcurrentUploads)
	assert.Equal(t, 30*time.Second, config.HealthCheckInterval)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default config",
			modify:      func(_ *Config) {},
			expectError: false,
		},
		{
			name:        "empty endpoint",
			modify:      func(c *Config) { c.Endpoint = "" },
			expectError: true,
			errorMsg:    "endpoint is required",
		},
		{
			name:        "empty access key",
			modify:      func(c *Config) { c.AccessKey = "" },
			expectError: true,
			errorMsg:    "access_key is required",
		},
		{
			name:        "empty secret key",
			modify:      func(c *Config) { c.SecretKey = "" },
			expectError: true,
			errorMsg:    "secret_key is required",
		},
		{
			name:        "zero connect timeout",
			modify:      func(c *Config) { c.ConnectTimeout = 0 },
			expectError: true,
			errorMsg:    "connect_timeout must be positive",
		},
		{
			name:        "negative connect timeout",
			modify:      func(c *Config) { c.ConnectTimeout = -1 },
			expectError: true,
			errorMsg:    "connect_timeout must be positive",
		},
		{
			name:        "negative request timeout",
			modify:      func(c *Config) { c.RequestTimeout = -1 },
			expectError: true,
			errorMsg:    "request_timeout must be positive",
		},
		{
			name:        "negative max retries",
			modify:      func(c *Config) { c.MaxRetries = -1 },
			expectError: true,
			errorMsg:    "max_retries cannot be negative",
		},
		{
			name:        "part size too small",
			modify:      func(c *Config) { c.PartSize = 1024 },
			expectError: true,
			errorMsg:    "part_size must be at least 5MB",
		},
		{
			name:        "zero concurrent uploads",
			modify:      func(c *Config) { c.ConcurrentUploads = 0 },
			expectError: true,
			errorMsg:    "concurrent_uploads must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.modify(config)

			err := config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultBucketConfig(t *testing.T) {
	config := DefaultBucketConfig("test-bucket")

	assert.Equal(t, "test-bucket", config.Name)
	assert.Equal(t, -1, config.RetentionDays)
	assert.False(t, config.Versioning)
	assert.False(t, config.ObjectLocking)
	assert.False(t, config.Public)
}

func TestBucketConfig_Chaining(t *testing.T) {
	tests := []struct {
		name      string
		build     func() *BucketConfig
		checkName string
		checkRet  int
		checkVer  bool
		checkLock bool
		checkPub  bool
	}{
		{
			name: "all options",
			build: func() *BucketConfig {
				return DefaultBucketConfig("chained").
					WithRetention(30).
					WithVersioning().
					WithObjectLocking().
					WithPublicAccess()
			},
			checkName: "chained",
			checkRet:  30,
			checkVer:  true,
			checkLock: true,
			checkPub:  true,
		},
		{
			name: "retention only",
			build: func() *BucketConfig {
				return DefaultBucketConfig("retention-only").
					WithRetention(90)
			},
			checkName: "retention-only",
			checkRet:  90,
			checkVer:  false,
			checkLock: false,
			checkPub:  false,
		},
		{
			name: "versioning only",
			build: func() *BucketConfig {
				return DefaultBucketConfig("ver-only").
					WithVersioning()
			},
			checkName: "ver-only",
			checkRet:  -1,
			checkVer:  true,
			checkLock: false,
			checkPub:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.build()
			assert.Equal(t, tt.checkName, config.Name)
			assert.Equal(t, tt.checkRet, config.RetentionDays)
			assert.Equal(t, tt.checkVer, config.Versioning)
			assert.Equal(t, tt.checkLock, config.ObjectLocking)
			assert.Equal(t, tt.checkPub, config.Public)
		})
	}
}

func TestDefaultLifecycleRule(t *testing.T) {
	rule := DefaultLifecycleRule("expire-old", 90)

	assert.Equal(t, "expire-old", rule.ID)
	assert.Equal(t, "", rule.Prefix)
	assert.True(t, rule.Enabled)
	assert.Equal(t, 90, rule.ExpirationDays)
	assert.Equal(t, 0, rule.NoncurrentDays)
	assert.False(t, rule.DeleteMarkerExpiry)
}

func TestLifecycleRule_Chaining(t *testing.T) {
	tests := []struct {
		name          string
		build         func() *LifecycleRule
		expectPrefix  string
		expectNonCurr int
		expectExpDays int
	}{
		{
			name: "with prefix and noncurrent",
			build: func() *LifecycleRule {
				return DefaultLifecycleRule("logs", 30).
					WithPrefix("logs/").
					WithNoncurrentExpiry(7)
			},
			expectPrefix:  "logs/",
			expectNonCurr: 7,
			expectExpDays: 30,
		},
		{
			name: "prefix only",
			build: func() *LifecycleRule {
				return DefaultLifecycleRule("data", 60).
					WithPrefix("data/")
			},
			expectPrefix:  "data/",
			expectNonCurr: 0,
			expectExpDays: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := tt.build()
			assert.Equal(t, tt.expectPrefix, rule.Prefix)
			assert.Equal(t, tt.expectNonCurr, rule.NoncurrentDays)
			assert.Equal(t, tt.expectExpDays, rule.ExpirationDays)
		})
	}
}
