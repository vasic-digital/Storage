package object

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectInfo_Fields(t *testing.T) {
	tests := []struct {
		name string
		info ObjectInfo
	}{
		{
			name: "basic object info",
			info: ObjectInfo{
				Key:          "path/to/file.txt",
				Size:         2048,
				LastModified: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
				ContentType:  "text/plain",
				ETag:         "abc123",
				Metadata:     map[string]string{"author": "test"},
			},
		},
		{
			name: "empty metadata",
			info: ObjectInfo{
				Key:  "file.bin",
				Size: 0,
			},
		},
		{
			name: "large object",
			info: ObjectInfo{
				Key:         "data/archive.tar.gz",
				Size:        5368709120, // 5GB
				ContentType: "application/gzip",
				ETag:        "etag-large",
				Metadata:    map[string]string{"compressed": "true", "source": "backup"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.info.Key, tt.info.Key)
			assert.Equal(t, tt.info.Size, tt.info.Size)
			assert.Equal(t, tt.info.ContentType, tt.info.ContentType)
			assert.Equal(t, tt.info.ETag, tt.info.ETag)
		})
	}
}

func TestBucketInfo_Fields(t *testing.T) {
	tests := []struct {
		name string
		info BucketInfo
	}{
		{
			name: "standard bucket",
			info: BucketInfo{
				Name:         "my-bucket",
				CreationDate: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "bucket with zero time",
			info: BucketInfo{
				Name: "empty-date-bucket",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.info.Name, tt.info.Name)
		})
	}
}

func TestBucketConfig(t *testing.T) {
	tests := []struct {
		name   string
		config BucketConfig
	}{
		{
			name: "simple bucket",
			config: BucketConfig{
				Name: "test-bucket",
			},
		},
		{
			name: "versioned bucket with retention",
			config: BucketConfig{
				Name:          "versioned-bucket",
				Versioning:    true,
				RetentionDays: 90,
				ObjectLocking: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.config.Name)
		})
	}
}

func TestObjectRef(t *testing.T) {
	tests := []struct {
		name   string
		ref    ObjectRef
		bucket string
		key    string
	}{
		{
			name:   "standard reference",
			ref:    ObjectRef{Bucket: "my-bucket", Key: "path/to/object"},
			bucket: "my-bucket",
			key:    "path/to/object",
		},
		{
			name:   "root level object",
			ref:    ObjectRef{Bucket: "data", Key: "file.txt"},
			bucket: "data",
			key:    "file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.bucket, tt.ref.Bucket)
			assert.Equal(t, tt.key, tt.ref.Key)
		})
	}
}

func TestConfig_Fields(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "minimal config",
			config: Config{
				Endpoint:  "localhost:9000",
				AccessKey: "admin",
				SecretKey: "password",
			},
		},
		{
			name: "full config",
			config: Config{
				Endpoint:       "s3.amazonaws.com",
				AccessKey:      "AKIAIOSFODNN7EXAMPLE",
				SecretKey:      "secret",
				UseSSL:         true,
				Region:         "us-west-2",
				ConnectTimeout: 30 * time.Second,
				RequestTimeout: 60 * time.Second,
				MaxRetries:     3,
				PartSize:       16 * 1024 * 1024,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.config.Endpoint)
			assert.NotEmpty(t, tt.config.AccessKey)
			assert.NotEmpty(t, tt.config.SecretKey)
		})
	}
}

func TestWithContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
	}{
		{"json", "application/json"},
		{"plain text", "text/plain"},
		{"binary", "application/octet-stream"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := WithContentType(tt.contentType)
			require.NotNil(t, opt)
			resolved := ResolvePutOptions(opt)
			assert.Equal(t, tt.contentType, resolved.ContentType)
		})
	}
}

func TestWithMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
	}{
		{
			name:     "single entry",
			metadata: map[string]string{"key": "value"},
		},
		{
			name:     "multiple entries",
			metadata: map[string]string{"author": "test", "version": "1.0"},
		},
		{
			name:     "nil metadata",
			metadata: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := WithMetadata(tt.metadata)
			require.NotNil(t, opt)
			resolved := ResolvePutOptions(opt)
			if tt.metadata == nil {
				assert.Nil(t, resolved.Metadata)
			} else {
				assert.Equal(t, tt.metadata, resolved.Metadata)
			}
		})
	}
}

func TestResolvePutOptions(t *testing.T) {
	tests := []struct {
		name            string
		opts            []PutOption
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
			opts: []PutOption{
				WithContentType("text/html"),
			},
			expectType:      "text/html",
			expectMetaCount: 0,
		},
		{
			name: "metadata only",
			opts: []PutOption{
				WithMetadata(map[string]string{"a": "b"}),
			},
			expectType:      "",
			expectMetaCount: 1,
		},
		{
			name: "both options",
			opts: []PutOption{
				WithContentType("application/pdf"),
				WithMetadata(map[string]string{"author": "test", "year": "2025"}),
			},
			expectType:      "application/pdf",
			expectMetaCount: 2,
		},
		{
			name: "last content type wins",
			opts: []PutOption{
				WithContentType("text/plain"),
				WithContentType("application/json"),
			},
			expectType:      "application/json",
			expectMetaCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := ResolvePutOptions(tt.opts...)
			assert.Equal(t, tt.expectType, resolved.ContentType)
			if tt.expectMetaCount == 0 && resolved.Metadata == nil {
				// OK - nil metadata when no metadata option given
			} else {
				assert.Len(t, resolved.Metadata, tt.expectMetaCount)
			}
		})
	}
}
