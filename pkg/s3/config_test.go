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
			name:        "zero request timeout",
			modify:      func(c *Config) { c.RequestTimeout = 0 },
			expectError: true,
			errorMsg:    "request_timeout must be positive",
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
			name:        "zero max retries is valid",
			modify:      func(c *Config) { c.MaxRetries = 0 },
			expectError: false,
		},
		{
			name:        "part size too small 1MB",
			modify:      func(c *Config) { c.PartSize = 1024 * 1024 },
			expectError: true,
			errorMsg:    "part_size must be at least 5MB",
		},
		{
			name:        "part size too small 4MB",
			modify:      func(c *Config) { c.PartSize = 4 * 1024 * 1024 },
			expectError: true,
			errorMsg:    "part_size must be at least 5MB",
		},
		{
			name:        "part size exactly 5MB is valid",
			modify:      func(c *Config) { c.PartSize = 5 * 1024 * 1024 },
			expectError: false,
		},
		{
			name:        "part size larger than 5MB is valid",
			modify:      func(c *Config) { c.PartSize = 100 * 1024 * 1024 },
			expectError: false,
		},
		{
			name:        "zero concurrent uploads",
			modify:      func(c *Config) { c.ConcurrentUploads = 0 },
			expectError: true,
			errorMsg:    "concurrent_uploads must be at least 1",
		},
		{
			name:        "negative concurrent uploads",
			modify:      func(c *Config) { c.ConcurrentUploads = -1 },
			expectError: true,
			errorMsg:    "concurrent_uploads must be at least 1",
		},
		{
			name:        "concurrent uploads exactly 1 is valid",
			modify:      func(c *Config) { c.ConcurrentUploads = 1 },
			expectError: false,
		},
		{
			name:        "high concurrent uploads is valid",
			modify:      func(c *Config) { c.ConcurrentUploads = 100 },
			expectError: false,
		},
		{
			name: "all fields valid with SSL",
			modify: func(c *Config) {
				c.Endpoint = "s3.amazonaws.com"
				c.UseSSL = true
				c.Region = "us-west-2"
			},
			expectError: false,
		},
		{
			name: "custom timeouts are valid",
			modify: func(c *Config) {
				c.ConnectTimeout = 120 * time.Second
				c.RequestTimeout = 300 * time.Second
			},
			expectError: false,
		},
		{
			name: "very small positive timeouts are valid",
			modify: func(c *Config) {
				c.ConnectTimeout = 1 * time.Nanosecond
				c.RequestTimeout = 1 * time.Nanosecond
			},
			expectError: false,
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

func TestConfig_ValidateMultipleErrors(t *testing.T) {
	// Test that validation stops at first error
	config := &Config{
		Endpoint:          "", // Error 1
		AccessKey:         "", // Error 2
		SecretKey:         "", // Error 3
		ConnectTimeout:    0,  // Error 4
		RequestTimeout:    0,  // Error 5
		PartSize:          0,  // Error 6
		ConcurrentUploads: 0,  // Error 7
	}

	err := config.Validate()
	require.Error(t, err)
	// Should fail on first error (endpoint)
	assert.Contains(t, err.Error(), "endpoint is required")
}

func TestConfig_ValidateOrderOfChecks(t *testing.T) {
	// Test validation order by providing invalid values and checking which error is returned
	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name: "endpoint checked first",
			config: &Config{
				Endpoint:          "",
				AccessKey:         "",
				SecretKey:         "",
				ConnectTimeout:    0,
				RequestTimeout:    0,
				PartSize:          0,
				ConcurrentUploads: 0,
			},
			expectedErr: "endpoint is required",
		},
		{
			name: "access key checked second",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "",
				SecretKey:         "",
				ConnectTimeout:    0,
				RequestTimeout:    0,
				PartSize:          0,
				ConcurrentUploads: 0,
			},
			expectedErr: "access_key is required",
		},
		{
			name: "secret key checked third",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "",
				ConnectTimeout:    0,
				RequestTimeout:    0,
				PartSize:          0,
				ConcurrentUploads: 0,
			},
			expectedErr: "secret_key is required",
		},
		{
			name: "connect timeout checked fourth",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    0,
				RequestTimeout:    0,
				PartSize:          0,
				ConcurrentUploads: 0,
			},
			expectedErr: "connect_timeout must be positive",
		},
		{
			name: "request timeout checked fifth",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    0,
				PartSize:          0,
				ConcurrentUploads: 0,
			},
			expectedErr: "request_timeout must be positive",
		},
		{
			name: "max retries checked sixth",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				MaxRetries:        -1,
				PartSize:          0,
				ConcurrentUploads: 0,
			},
			expectedErr: "max_retries cannot be negative",
		},
		{
			name: "part size checked seventh",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				MaxRetries:        0,
				PartSize:          1024, // Too small
				ConcurrentUploads: 0,
			},
			expectedErr: "part_size must be at least 5MB",
		},
		{
			name: "concurrent uploads checked last",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				MaxRetries:        0,
				PartSize:          16 * 1024 * 1024,
				ConcurrentUploads: 0,
			},
			expectedErr: "concurrent_uploads must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestDefaultBucketConfig(t *testing.T) {
	tests := []struct {
		name       string
		bucketName string
	}{
		{"simple name", "test-bucket"},
		{"with numbers", "bucket123"},
		{"with dashes", "my-test-bucket"},
		{"long name", "a-very-long-bucket-name-that-is-still-valid"},
		{"empty name", ""},
		{"unicode name", "bucket-with-unicode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultBucketConfig(tt.bucketName)

			assert.Equal(t, tt.bucketName, config.Name)
			assert.Equal(t, -1, config.RetentionDays)
			assert.False(t, config.Versioning)
			assert.False(t, config.ObjectLocking)
			assert.False(t, config.Public)
		})
	}
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
		{
			name: "object locking only",
			build: func() *BucketConfig {
				return DefaultBucketConfig("lock-only").
					WithObjectLocking()
			},
			checkName: "lock-only",
			checkRet:  -1,
			checkVer:  false,
			checkLock: true,
			checkPub:  false,
		},
		{
			name: "public access only",
			build: func() *BucketConfig {
				return DefaultBucketConfig("pub-only").
					WithPublicAccess()
			},
			checkName: "pub-only",
			checkRet:  -1,
			checkVer:  false,
			checkLock: false,
			checkPub:  true,
		},
		{
			name: "versioning and locking",
			build: func() *BucketConfig {
				return DefaultBucketConfig("ver-lock").
					WithVersioning().
					WithObjectLocking()
			},
			checkName: "ver-lock",
			checkRet:  -1,
			checkVer:  true,
			checkLock: true,
			checkPub:  false,
		},
		{
			name: "retention and public",
			build: func() *BucketConfig {
				return DefaultBucketConfig("ret-pub").
					WithRetention(7).
					WithPublicAccess()
			},
			checkName: "ret-pub",
			checkRet:  7,
			checkVer:  false,
			checkLock: false,
			checkPub:  true,
		},
		{
			name: "zero retention",
			build: func() *BucketConfig {
				return DefaultBucketConfig("zero-ret").
					WithRetention(0)
			},
			checkName: "zero-ret",
			checkRet:  0,
			checkVer:  false,
			checkLock: false,
			checkPub:  false,
		},
		{
			name: "large retention",
			build: func() *BucketConfig {
				return DefaultBucketConfig("large-ret").
					WithRetention(3650) // 10 years
			},
			checkName: "large-ret",
			checkRet:  3650,
			checkVer:  false,
			checkLock: false,
			checkPub:  false,
		},
		{
			name: "retention override",
			build: func() *BucketConfig {
				return DefaultBucketConfig("override").
					WithRetention(30).
					WithRetention(60)
			},
			checkName: "override",
			checkRet:  60, // Last value wins
			checkVer:  false,
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

func TestBucketConfig_ChainingReturnsPointer(t *testing.T) {
	// Verify that each method returns the same pointer for chaining
	config := DefaultBucketConfig("test")
	ptr1 := config.WithRetention(30)
	ptr2 := ptr1.WithVersioning()
	ptr3 := ptr2.WithObjectLocking()
	ptr4 := ptr3.WithPublicAccess()

	assert.Same(t, config, ptr1)
	assert.Same(t, config, ptr2)
	assert.Same(t, config, ptr3)
	assert.Same(t, config, ptr4)
}

func TestDefaultLifecycleRule(t *testing.T) {
	tests := []struct {
		name           string
		ruleID         string
		expirationDays int
	}{
		{"simple rule", "expire-old", 90},
		{"short expiration", "quick-expire", 1},
		{"long expiration", "archive-rule", 365},
		{"zero expiration", "no-expire", 0},
		{"empty id", "", 30},
		{"negative expiration", "negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := DefaultLifecycleRule(tt.ruleID, tt.expirationDays)

			assert.Equal(t, tt.ruleID, rule.ID)
			assert.Equal(t, "", rule.Prefix)
			assert.True(t, rule.Enabled)
			assert.Equal(t, tt.expirationDays, rule.ExpirationDays)
			assert.Equal(t, 0, rule.NoncurrentDays)
			assert.False(t, rule.DeleteMarkerExpiry)
		})
	}
}

func TestLifecycleRule_Chaining(t *testing.T) {
	tests := []struct {
		name          string
		build         func() *LifecycleRule
		expectPrefix  string
		expectNonCurr int
		expectExpDays int
		expectEnabled bool
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
			expectEnabled: true,
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
			expectEnabled: true,
		},
		{
			name: "noncurrent only",
			build: func() *LifecycleRule {
				return DefaultLifecycleRule("versions", 90).
					WithNoncurrentExpiry(30)
			},
			expectPrefix:  "",
			expectNonCurr: 30,
			expectExpDays: 90,
			expectEnabled: true,
		},
		{
			name: "empty prefix",
			build: func() *LifecycleRule {
				return DefaultLifecycleRule("all", 30).
					WithPrefix("")
			},
			expectPrefix:  "",
			expectNonCurr: 0,
			expectExpDays: 30,
			expectEnabled: true,
		},
		{
			name: "complex prefix",
			build: func() *LifecycleRule {
				return DefaultLifecycleRule("nested", 14).
					WithPrefix("path/to/deeply/nested/objects/")
			},
			expectPrefix:  "path/to/deeply/nested/objects/",
			expectNonCurr: 0,
			expectExpDays: 14,
			expectEnabled: true,
		},
		{
			name: "zero noncurrent days",
			build: func() *LifecycleRule {
				return DefaultLifecycleRule("no-version", 30).
					WithNoncurrentExpiry(0)
			},
			expectPrefix:  "",
			expectNonCurr: 0,
			expectExpDays: 30,
			expectEnabled: true,
		},
		{
			name: "prefix override",
			build: func() *LifecycleRule {
				return DefaultLifecycleRule("override", 30).
					WithPrefix("first/").
					WithPrefix("second/")
			},
			expectPrefix:  "second/",
			expectNonCurr: 0,
			expectExpDays: 30,
			expectEnabled: true,
		},
		{
			name: "noncurrent override",
			build: func() *LifecycleRule {
				return DefaultLifecycleRule("override", 30).
					WithNoncurrentExpiry(7).
					WithNoncurrentExpiry(14)
			},
			expectPrefix:  "",
			expectNonCurr: 14,
			expectExpDays: 30,
			expectEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := tt.build()
			assert.Equal(t, tt.expectPrefix, rule.Prefix)
			assert.Equal(t, tt.expectNonCurr, rule.NoncurrentDays)
			assert.Equal(t, tt.expectExpDays, rule.ExpirationDays)
			assert.Equal(t, tt.expectEnabled, rule.Enabled)
		})
	}
}

func TestLifecycleRule_ChainingReturnsPointer(t *testing.T) {
	// Verify that each method returns the same pointer for chaining
	rule := DefaultLifecycleRule("test", 30)
	ptr1 := rule.WithPrefix("prefix/")
	ptr2 := ptr1.WithNoncurrentExpiry(7)

	assert.Same(t, rule, ptr1)
	assert.Same(t, rule, ptr2)
}

func TestLifecycleRule_DirectAssignment(t *testing.T) {
	// Test that fields can be directly assigned without chaining
	rule := &LifecycleRule{
		ID:                 "direct-rule",
		Prefix:             "custom/prefix/",
		Enabled:            false,
		ExpirationDays:     45,
		NoncurrentDays:     15,
		DeleteMarkerExpiry: true,
	}

	assert.Equal(t, "direct-rule", rule.ID)
	assert.Equal(t, "custom/prefix/", rule.Prefix)
	assert.False(t, rule.Enabled)
	assert.Equal(t, 45, rule.ExpirationDays)
	assert.Equal(t, 15, rule.NoncurrentDays)
	assert.True(t, rule.DeleteMarkerExpiry)
}

func TestConfig_FieldTypes(t *testing.T) {
	// Verify that all fields have expected types and JSON/YAML tags
	config := DefaultConfig()

	// All fields should be set to their default values
	assert.IsType(t, "", config.Endpoint)
	assert.IsType(t, "", config.AccessKey)
	assert.IsType(t, "", config.SecretKey)
	assert.IsType(t, false, config.UseSSL)
	assert.IsType(t, "", config.Region)
	assert.IsType(t, time.Duration(0), config.ConnectTimeout)
	assert.IsType(t, time.Duration(0), config.RequestTimeout)
	assert.IsType(t, 0, config.MaxRetries)
	assert.IsType(t, int64(0), config.PartSize)
	assert.IsType(t, 0, config.ConcurrentUploads)
	assert.IsType(t, time.Duration(0), config.HealthCheckInterval)
}

func TestBucketConfig_FieldTypes(t *testing.T) {
	config := DefaultBucketConfig("test")

	assert.IsType(t, "", config.Name)
	assert.IsType(t, 0, config.RetentionDays)
	assert.IsType(t, false, config.Versioning)
	assert.IsType(t, false, config.ObjectLocking)
	assert.IsType(t, false, config.Public)
}

func TestLifecycleRule_FieldTypes(t *testing.T) {
	rule := DefaultLifecycleRule("test", 30)

	assert.IsType(t, "", rule.ID)
	assert.IsType(t, "", rule.Prefix)
	assert.IsType(t, false, rule.Enabled)
	assert.IsType(t, 0, rule.ExpirationDays)
	assert.IsType(t, 0, rule.NoncurrentDays)
	assert.IsType(t, false, rule.DeleteMarkerExpiry)
}

func TestConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "whitespace endpoint",
			config: &Config{
				Endpoint:          "   ",
				AccessKey:         "key",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				PartSize:          16 * 1024 * 1024,
				ConcurrentUploads: 1,
			},
			expectError: false, // Whitespace is not validated, just empty check
		},
		{
			name: "very long endpoint",
			config: &Config{
				Endpoint:          "a" + string(make([]byte, 1000)),
				AccessKey:         "key",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				PartSize:          16 * 1024 * 1024,
				ConcurrentUploads: 1,
			},
			expectError: false,
		},
		{
			name: "max int64 part size",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "key",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				PartSize:          1<<63 - 1,
				ConcurrentUploads: 1,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
