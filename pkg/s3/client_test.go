package s3

import (
	"bytes"
	"context"
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

func TestClient_IsConnected(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
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

func TestClient_Concurrency(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)

	done := make(chan bool, 20)

	for i := 0; i < 10; i++ {
		go func() {
			_ = client.IsConnected()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			_ = client.Close()
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := object.ResolvePutOptions(tt.opts...)
			assert.Equal(t, tt.expectType, resolved.ContentType)
			if tt.expectMetaCount == 0 && resolved.Metadata == nil {
				return
			}
			assert.Len(t, resolved.Metadata, tt.expectMetaCount)
		})
	}
}
