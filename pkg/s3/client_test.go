package s3

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.storage/pkg/object"
)


func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		logger      *logrus.Logger
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil config uses defaults",
			config:      nil,
			logger:      nil,
			expectError: false,
		},
		{
			name: "valid custom config",
			config: &Config{
				Endpoint:          "minio.example.com:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    60 * time.Second,
				RequestTimeout:    120 * time.Second,
				PartSize:          16 * 1024 * 1024,
				ConcurrentUploads: 4,
			},
			logger:      logrus.New(),
			expectError: false,
		},
		{
			name: "valid config with SSL",
			config: &Config{
				Endpoint:          "s3.amazonaws.com",
				AccessKey:         "aws-access",
				SecretKey:         "aws-secret",
				UseSSL:            true,
				Region:            "us-west-2",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				PartSize:          32 * 1024 * 1024,
				ConcurrentUploads: 8,
			},
			logger:      nil,
			expectError: false,
		},
		{
			name: "empty endpoint",
			config: &Config{
				Endpoint:  "",
				AccessKey: "access",
				SecretKey: "secret",
			},
			logger:      nil,
			expectError: true,
			errorMsg:    "endpoint is required",
		},
		{
			name: "empty access key",
			config: &Config{
				Endpoint:  "localhost:9000",
				AccessKey: "",
				SecretKey: "secret",
			},
			logger:      nil,
			expectError: true,
			errorMsg:    "access_key is required",
		},
		{
			name: "empty secret key",
			config: &Config{
				Endpoint:  "localhost:9000",
				AccessKey: "access",
				SecretKey: "",
			},
			logger:      nil,
			expectError: true,
			errorMsg:    "invalid config",
		},
		{
			name: "invalid connect timeout",
			config: &Config{
				Endpoint:       "localhost:9000",
				AccessKey:      "access",
				SecretKey:      "secret",
				ConnectTimeout: 0,
			},
			logger:      nil,
			expectError: true,
			errorMsg:    "connect_timeout",
		},
		{
			name: "negative connect timeout",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    -1 * time.Second,
				RequestTimeout:    60 * time.Second,
				PartSize:          16 * 1024 * 1024,
				ConcurrentUploads: 4,
			},
			logger:      nil,
			expectError: true,
			errorMsg:    "connect_timeout must be positive",
		},
		{
			name: "invalid request timeout",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    0,
				PartSize:          16 * 1024 * 1024,
				ConcurrentUploads: 4,
			},
			logger:      nil,
			expectError: true,
			errorMsg:    "request_timeout must be positive",
		},
		{
			name: "negative max retries",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				MaxRetries:        -1,
				PartSize:          16 * 1024 * 1024,
				ConcurrentUploads: 4,
			},
			logger:      nil,
			expectError: true,
			errorMsg:    "max_retries cannot be negative",
		},
		{
			name: "part size too small",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				PartSize:          1024 * 1024, // 1MB - too small
				ConcurrentUploads: 4,
			},
			logger:      nil,
			expectError: true,
			errorMsg:    "part_size must be at least 5MB",
		},
		{
			name: "zero concurrent uploads",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				PartSize:          16 * 1024 * 1024,
				ConcurrentUploads: 0,
			},
			logger:      nil,
			expectError: true,
			errorMsg:    "concurrent_uploads must be at least 1",
		},
		{
			name: "config with zero max retries is valid",
			config: &Config{
				Endpoint:          "localhost:9000",
				AccessKey:         "access",
				SecretKey:         "secret",
				ConnectTimeout:    30 * time.Second,
				RequestTimeout:    60 * time.Second,
				MaxRetries:        0,
				PartSize:          16 * 1024 * 1024,
				ConcurrentUploads: 4,
			},
			logger:      nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config, tt.logger)
			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, client)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
				assert.False(t, client.IsConnected())
			}
		})
	}
}

func TestNewClient_LoggerInitialization(t *testing.T) {
	// Test that logger is created if nil
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.logger)

	// Test with explicit logger
	logger := logrus.New()
	client2, err := NewClient(nil, logger)
	require.NoError(t, err)
	assert.Same(t, logger, client2.logger)
}

func TestClient_Close(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"close disconnected client"},
		{"close is idempotent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(nil, nil)
			require.NoError(t, err)

			err = client.Close()
			assert.NoError(t, err)
			assert.False(t, client.IsConnected())

			// Second close should also work
			err = client.Close()
			assert.NoError(t, err)
		})
	}
}

func TestClient_Close_ClearsMinioClient(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)

	// Manually set minioClient to simulate connected state
	client.mu.Lock()
	client.connected = true
	client.mu.Unlock()

	err = client.Close()
	assert.NoError(t, err)
	assert.False(t, client.IsConnected())
	assert.Nil(t, client.minioClient)
}

func TestClient_IsConnected(t *testing.T) {
	tests := []struct {
		name      string
		connected bool
		expected  bool
	}{
		{"not connected", false, false},
		{"connected", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(nil, nil)
			require.NoError(t, err)

			client.mu.Lock()
			client.connected = tt.connected
			client.mu.Unlock()

			assert.Equal(t, tt.expected, client.IsConnected())
		})
	}
}

func TestClient_OperationsWhenNotConnected(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	errTests := []struct {
		name string
		fn   func() error
	}{
		{
			"HealthCheck",
			func() error { return client.HealthCheck(ctx) },
		},
		{
			"CreateBucket",
			func() error {
				return client.CreateBucket(ctx, object.BucketConfig{Name: "t"})
			},
		},
		{
			"DeleteBucket",
			func() error { return client.DeleteBucket(ctx, "t") },
		},
		{
			"PutObject",
			func() error {
				return client.PutObject(
					ctx, "b", "k",
					bytes.NewReader([]byte("data")), 4,
				)
			},
		},
		{
			"DeleteObject",
			func() error { return client.DeleteObject(ctx, "b", "k") },
		},
		{
			"CopyObject",
			func() error {
				return client.CopyObject(
					ctx,
					object.ObjectRef{Bucket: "s", Key: "k"},
					object.ObjectRef{Bucket: "d", Key: "k"},
				)
			},
		},
		{
			"SetLifecycleRule",
			func() error {
				return client.SetLifecycleRule(
					ctx, "b", DefaultLifecycleRule("id", 30),
				)
			},
		},
		{
			"RemoveLifecycleRule",
			func() error {
				return client.RemoveLifecycleRule(ctx, "b", "id")
			},
		},
	}

	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not connected")
		})
	}

	valTests := []struct {
		name string
		fn   func() error
	}{
		{
			"ListBuckets",
			func() error {
				buckets, err := client.ListBuckets(ctx)
				assert.Nil(t, buckets)
				return err
			},
		},
		{
			"BucketExists",
			func() error {
				exists, err := client.BucketExists(ctx, "t")
				assert.False(t, exists)
				return err
			},
		},
		{
			"GetObject",
			func() error {
				obj, err := client.GetObject(ctx, "b", "k")
				assert.Nil(t, obj)
				return err
			},
		},
		{
			"ListObjects",
			func() error {
				objs, err := client.ListObjects(ctx, "b", "p")
				assert.Nil(t, objs)
				return err
			},
		},
		{
			"StatObject",
			func() error {
				info, err := client.StatObject(ctx, "b", "k")
				assert.Nil(t, info)
				return err
			},
		},
		{
			"GetPresignedURL",
			func() error {
				u, err := client.GetPresignedURL(
					ctx, "b", "k", time.Hour,
				)
				assert.Empty(t, u)
				return err
			},
		},
		{
			"GetPresignedPutURL",
			func() error {
				u, err := client.GetPresignedPutURL(
					ctx, "b", "k", time.Hour,
				)
				assert.Empty(t, u)
				return err
			},
		},
	}

	for _, tt := range valTests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not connected")
		})
	}
}

func TestClient_OperationsWhenMinioClientNil(t *testing.T) {
	// Test case where connected is true but minioClient is nil
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	// Set connected to true but leave minioClient nil
	client.mu.Lock()
	client.connected = true
	client.minioClient = nil
	client.mu.Unlock()

	// All operations should still fail
	err = client.HealthCheck(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	_, err = client.ListBuckets(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClient_Concurrency(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)

	done := make(chan bool, 40)

	// Concurrent IsConnected calls
	for i := 0; i < 10; i++ {
		go func() {
			_ = client.IsConnected()
			done <- true
		}()
	}

	// Concurrent Close calls
	for i := 0; i < 10; i++ {
		go func() {
			_ = client.Close()
			done <- true
		}()
	}

	// Concurrent operation attempts (will fail but should not race)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		go func() {
			_ = client.HealthCheck(ctx)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			_, _ = client.ListBuckets(ctx)
			done <- true
		}()
	}

	for i := 0; i < 40; i++ {
		<-done
	}
}

func TestClient_ConcurrencyWithMixedOperations(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	var wg sync.WaitGroup
	operations := 100

	wg.Add(operations * 5)

	for i := 0; i < operations; i++ {
		go func() {
			defer wg.Done()
			_ = client.IsConnected()
		}()

		go func() {
			defer wg.Done()
			_ = client.HealthCheck(ctx)
		}()

		go func() {
			defer wg.Done()
			_, _ = client.BucketExists(ctx, "test")
		}()

		go func() {
			defer wg.Done()
			_, _ = client.ListObjects(ctx, "test", "")
		}()

		go func() {
			defer wg.Done()
			_ = client.Close()
		}()
	}

	wg.Wait()
}

func TestClient_PutOptions(t *testing.T) {
	tests := []struct {
		name            string
		opts            []object.PutOption
		expectType      string
		expectMetaCount int
	}{
		{
			name:            "no options",
			opts:            nil,
			expectType:      "",
			expectMetaCount: 0,
		},
		{
			name: "content type only",
			opts: []object.PutOption{
				object.WithContentType("application/json"),
			},
			expectType:      "application/json",
			expectMetaCount: 0,
		},
		{
			name: "metadata only",
			opts: []object.PutOption{
				object.WithMetadata(map[string]string{"k": "v"}),
			},
			expectType:      "",
			expectMetaCount: 1,
		},
		{
			name: "both options",
			opts: []object.PutOption{
				object.WithContentType("text/plain"),
				object.WithMetadata(map[string]string{"a": "b", "c": "d"}),
			},
			expectType:      "text/plain",
			expectMetaCount: 2,
		},
		{
			name: "multiple metadata entries",
			opts: []object.PutOption{
				object.WithMetadata(map[string]string{
					"author":  "test",
					"version": "1.0",
					"tags":    "important",
				}),
			},
			expectType:      "",
			expectMetaCount: 3,
		},
		{
			name: "content type override",
			opts: []object.PutOption{
				object.WithContentType("text/plain"),
				object.WithContentType("application/octet-stream"),
			},
			expectType:      "application/octet-stream",
			expectMetaCount: 0,
		},
		{
			name: "metadata override",
			opts: []object.PutOption{
				object.WithMetadata(map[string]string{"k": "v1"}),
				object.WithMetadata(map[string]string{"k": "v2", "k2": "v3"}),
			},
			expectType:      "",
			expectMetaCount: 2, // Last one wins
		},
		{
			name: "empty content type",
			opts: []object.PutOption{
				object.WithContentType(""),
			},
			expectType:      "",
			expectMetaCount: 0,
		},
		{
			name: "nil metadata",
			opts: []object.PutOption{
				object.WithMetadata(nil),
			},
			expectType:      "",
			expectMetaCount: 0,
		},
		{
			name: "empty metadata map",
			opts: []object.PutOption{
				object.WithMetadata(map[string]string{}),
			},
			expectType:      "",
			expectMetaCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := object.ResolvePutOptions(tt.opts...)
			assert.Equal(t, tt.expectType, resolved.ContentType)
			if tt.expectMetaCount == 0 {
				if resolved.Metadata != nil {
					assert.Len(t, resolved.Metadata, 0)
				}
				return
			}
			assert.Len(t, resolved.Metadata, tt.expectMetaCount)
		})
	}
}

func TestClient_ConnectFailure(t *testing.T) {
	// Test with an unreachable endpoint
	config := &Config{
		Endpoint:          "unreachable.invalid.endpoint:9999",
		AccessKey:         "test",
		SecretKey:         "test",
		ConnectTimeout:    1 * time.Second,
		RequestTimeout:    1 * time.Second,
		PartSize:          16 * 1024 * 1024,
		ConcurrentUploads: 1,
	}

	client, err := NewClient(config, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
	assert.False(t, client.IsConnected())
}

func TestClient_InterfaceCompliance(t *testing.T) {
	// Verify that Client implements required interfaces at compile time
	var _ object.ObjectStore = (*Client)(nil)
	var _ object.BucketManager = (*Client)(nil)
}

func TestClient_ContextCancellation(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Operations should fail quickly with not connected error
	// (since we're not actually connected, the context doesn't matter)
	err = client.HealthCheck(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClient_LifecycleRuleVariations(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	tests := []struct {
		name string
		rule *LifecycleRule
	}{
		{
			name: "enabled rule with expiration",
			rule: &LifecycleRule{
				ID:             "test-rule",
				Enabled:        true,
				ExpirationDays: 30,
			},
		},
		{
			name: "disabled rule",
			rule: &LifecycleRule{
				ID:             "disabled-rule",
				Enabled:        false,
				ExpirationDays: 30,
			},
		},
		{
			name: "rule with prefix",
			rule: &LifecycleRule{
				ID:             "prefix-rule",
				Prefix:         "logs/",
				Enabled:        true,
				ExpirationDays: 7,
			},
		},
		{
			name: "rule with noncurrent days",
			rule: &LifecycleRule{
				ID:             "noncurrent-rule",
				Enabled:        true,
				NoncurrentDays: 30,
			},
		},
		{
			name: "rule with delete marker expiry",
			rule: &LifecycleRule{
				ID:                 "marker-rule",
				Enabled:            true,
				DeleteMarkerExpiry: true,
			},
		},
		{
			name: "rule with all options",
			rule: &LifecycleRule{
				ID:                 "full-rule",
				Prefix:             "archive/",
				Enabled:            true,
				ExpirationDays:     90,
				NoncurrentDays:     30,
				DeleteMarkerExpiry: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All operations should fail since not connected
			err := client.SetLifecycleRule(ctx, "bucket", tt.rule)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not connected")
		})
	}
}

func TestClient_ObjectRefOperations(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	tests := []struct {
		name string
		src  object.ObjectRef
		dst  object.ObjectRef
	}{
		{
			name: "same bucket copy",
			src:  object.ObjectRef{Bucket: "bucket", Key: "source.txt"},
			dst:  object.ObjectRef{Bucket: "bucket", Key: "dest.txt"},
		},
		{
			name: "cross bucket copy",
			src:  object.ObjectRef{Bucket: "source-bucket", Key: "file.txt"},
			dst:  object.ObjectRef{Bucket: "dest-bucket", Key: "file.txt"},
		},
		{
			name: "nested path copy",
			src:  object.ObjectRef{Bucket: "bucket", Key: "path/to/source.txt"},
			dst:  object.ObjectRef{Bucket: "bucket", Key: "new/path/dest.txt"},
		},
		{
			name: "empty key",
			src:  object.ObjectRef{Bucket: "bucket", Key: ""},
			dst:  object.ObjectRef{Bucket: "bucket", Key: "dest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.CopyObject(ctx, tt.src, tt.dst)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not connected")
		})
	}
}

func TestClient_PresignedURLDurations(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	durations := []time.Duration{
		1 * time.Minute,
		15 * time.Minute,
		1 * time.Hour,
		24 * time.Hour,
		7 * 24 * time.Hour,
	}

	for _, d := range durations {
		t.Run(d.String(), func(t *testing.T) {
			url, err := client.GetPresignedURL(ctx, "bucket", "key", d)
			require.Error(t, err)
			assert.Empty(t, url)
			assert.Contains(t, err.Error(), "not connected")

			url, err = client.GetPresignedPutURL(ctx, "bucket", "key", d)
			require.Error(t, err)
			assert.Empty(t, url)
			assert.Contains(t, err.Error(), "not connected")
		})
	}
}

func TestClient_ListObjectsWithPrefixes(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	prefixes := []string{
		"",
		"prefix/",
		"path/to/objects/",
		"2024/01/",
		"uploads/user-123/",
	}

	for _, prefix := range prefixes {
		t.Run("prefix:"+prefix, func(t *testing.T) {
			objs, err := client.ListObjects(ctx, "bucket", prefix)
			require.Error(t, err)
			assert.Nil(t, objs)
			assert.Contains(t, err.Error(), "not connected")
		})
	}
}

func TestClient_PutObjectWithVariousSizes(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	sizes := []int64{
		0,
		1,
		1024,
		1024 * 1024,
		-1, // Unknown size
	}

	for _, size := range sizes {
		t.Run("size:"+string(rune(size)), func(t *testing.T) {
			data := make([]byte, 0)
			if size > 0 {
				data = make([]byte, size)
			}
			err := client.PutObject(ctx, "bucket", "key", bytes.NewReader(data), size)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not connected")
		})
	}
}

func TestClient_PutObjectWithAllOptions(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	data := []byte("test content")
	opts := []object.PutOption{
		object.WithContentType("application/json"),
		object.WithMetadata(map[string]string{
			"x-amz-meta-author": "test",
			"x-amz-meta-app":    "test-app",
		}),
	}

	err = client.PutObject(ctx, "bucket", "key", bytes.NewReader(data), int64(len(data)), opts...)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClient_BucketConfigVariations(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	configs := []object.BucketConfig{
		{Name: "simple-bucket"},
		{Name: "versioned-bucket", Versioning: true},
		{Name: "retention-bucket", RetentionDays: 30},
		{Name: "locked-bucket", ObjectLocking: true},
		{Name: "full-config", Versioning: true, RetentionDays: 90, ObjectLocking: true},
	}

	for _, cfg := range configs {
		t.Run(cfg.Name, func(t *testing.T) {
			err := client.CreateBucket(ctx, cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not connected")
		})
	}
}

func TestClient_GetObjectReturnsReadCloser(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	reader, err := client.GetObject(ctx, "bucket", "key")
	require.Error(t, err)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClient_StatObjectReturnsObjectInfo(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	info, err := client.StatObject(ctx, "bucket", "key")
	require.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClient_NewClientWithCustomLogger(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	client, err := NewClient(nil, logger)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Same(t, logger, client.logger)
}

func TestClient_ConfigWithAllFields(t *testing.T) {
	config := &Config{
		Endpoint:            "custom.endpoint.com:443",
		AccessKey:           "AKIAIOSFODNN7EXAMPLE",
		SecretKey:           "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		UseSSL:              true,
		Region:              "eu-west-1",
		ConnectTimeout:      45 * time.Second,
		RequestTimeout:      90 * time.Second,
		MaxRetries:          5,
		PartSize:            64 * 1024 * 1024,
		ConcurrentUploads:   8,
		HealthCheckInterval: 60 * time.Second,
	}

	client, err := NewClient(config, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, config.Endpoint, client.config.Endpoint)
	assert.Equal(t, config.AccessKey, client.config.AccessKey)
	assert.Equal(t, config.UseSSL, client.config.UseSSL)
	assert.Equal(t, config.Region, client.config.Region)
	assert.Equal(t, config.PartSize, client.config.PartSize)
}

func TestClient_ReaderVariations(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	ctx := context.Background()

	readers := []struct {
		name   string
		reader io.Reader
		size   int64
	}{
		{"empty bytes", bytes.NewReader([]byte{}), 0},
		{"string reader", strings.NewReader("test"), 4},
		{"bytes reader", bytes.NewReader([]byte{1, 2, 3, 4, 5}), 5},
		{"nil reader", nil, 0},
	}

	for _, r := range readers {
		t.Run(r.name, func(t *testing.T) {
			err := client.PutObject(ctx, "bucket", "key", r.reader, r.size)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not connected")
		})
	}
}
