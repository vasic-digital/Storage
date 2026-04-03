package object_test

import (
	"strings"
	"testing"
	"time"

	"digital.vasic.storage/pkg/object"
	"github.com/stretchr/testify/assert"
)

// --- ObjectRef Edge Cases ---

func TestObjectRef_EmptyBucketAndKey(t *testing.T) {
	t.Parallel()
	ref := object.ObjectRef{}
	assert.Empty(t, ref.Bucket)
	assert.Empty(t, ref.Key)
}

func TestObjectRef_KeyWithSpecialCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
	}{
		{"spaces", "file with spaces.txt"},
		{"unicode", "\u4e2d\u6587\u6587\u4ef6.txt"},
		{"emoji", "\U0001f4c4document.pdf"},
		{"slashes", "deep/nested/path/file.txt"},
		{"dots", "many...dots...file.txt"},
		{"backslash", "path\\to\\file.txt"},
		{"hash", "file#1.txt"},
		{"at_sign", "user@domain/file.txt"},
		{"percent", "100%done.txt"},
		{"null_byte", "file\x00name.txt"},
		{"newline", "file\nname.txt"},
		{"tab", "file\tname.txt"},
		{"single_quote", "file'name.txt"},
		{"double_quote", "file\"name.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ref := object.ObjectRef{
				Bucket: "test-bucket",
				Key:    tt.key,
			}
			assert.Equal(t, tt.key, ref.Key)
		})
	}
}

func TestObjectRef_VeryLongKey(t *testing.T) {
	t.Parallel()
	longKey := strings.Repeat("a/", 500) + "file.txt"
	ref := object.ObjectRef{
		Bucket: "bucket",
		Key:    longKey,
	}
	assert.True(t, len(ref.Key) > 1000)
}

// --- ObjectInfo Edge Cases ---

func TestObjectInfo_ZeroValues(t *testing.T) {
	t.Parallel()
	info := object.ObjectInfo{}
	assert.Empty(t, info.Key)
	assert.Equal(t, int64(0), info.Size)
	assert.True(t, info.LastModified.IsZero())
	assert.Empty(t, info.ContentType)
	assert.Empty(t, info.ETag)
	assert.Nil(t, info.Metadata)
}

func TestObjectInfo_NegativeSize(t *testing.T) {
	t.Parallel()
	info := object.ObjectInfo{
		Key:  "test.txt",
		Size: -1,
	}
	assert.Equal(t, int64(-1), info.Size)
}

func TestObjectInfo_NilMetadata(t *testing.T) {
	t.Parallel()
	info := object.ObjectInfo{
		Key:      "test.txt",
		Metadata: nil,
	}
	assert.Nil(t, info.Metadata)
}

func TestObjectInfo_EmptyMetadata(t *testing.T) {
	t.Parallel()
	info := object.ObjectInfo{
		Key:      "test.txt",
		Metadata: map[string]string{},
	}
	assert.Empty(t, info.Metadata)
}

func TestObjectInfo_FutureLastModified(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(365 * 24 * time.Hour)
	info := object.ObjectInfo{
		Key:          "future.txt",
		LastModified: future,
	}
	assert.True(t, info.LastModified.After(time.Now()))
}

// --- BucketConfig Edge Cases ---

func TestBucketConfig_EmptyName(t *testing.T) {
	t.Parallel()
	cfg := object.BucketConfig{}
	assert.Empty(t, cfg.Name)
}

func TestBucketConfig_NegativeRetentionDays(t *testing.T) {
	t.Parallel()
	cfg := object.BucketConfig{
		Name:          "test",
		RetentionDays: -1,
	}
	assert.Equal(t, -1, cfg.RetentionDays)
}

func TestBucketConfig_AllOptionsEnabled(t *testing.T) {
	t.Parallel()
	cfg := object.BucketConfig{
		Name:          "all-features",
		Versioning:    true,
		RetentionDays: 30,
		ObjectLocking: true,
	}
	assert.True(t, cfg.Versioning)
	assert.True(t, cfg.ObjectLocking)
	assert.Equal(t, 30, cfg.RetentionDays)
}

// --- BucketInfo Edge Cases ---

func TestBucketInfo_ZeroCreationDate(t *testing.T) {
	t.Parallel()
	info := object.BucketInfo{}
	assert.True(t, info.CreationDate.IsZero())
}

// --- Config Edge Cases ---

func TestConfig_ZeroValues(t *testing.T) {
	t.Parallel()
	cfg := object.Config{}
	assert.Empty(t, cfg.Endpoint)
	assert.Empty(t, cfg.AccessKey)
	assert.Empty(t, cfg.SecretKey)
	assert.False(t, cfg.UseSSL)
	assert.Empty(t, cfg.Region)
	assert.Equal(t, time.Duration(0), cfg.ConnectTimeout)
	assert.Equal(t, time.Duration(0), cfg.RequestTimeout)
	assert.Equal(t, 0, cfg.MaxRetries)
	assert.Equal(t, int64(0), cfg.PartSize)
}

func TestConfig_NegativeRetries(t *testing.T) {
	t.Parallel()
	cfg := object.Config{
		MaxRetries: -1,
	}
	assert.Equal(t, -1, cfg.MaxRetries)
}

func TestConfig_NegativePartSize(t *testing.T) {
	t.Parallel()
	cfg := object.Config{
		PartSize: -1,
	}
	assert.Equal(t, int64(-1), cfg.PartSize)
}

// --- PutOption Functional Options ---

func TestResolvePutOptions_NoOptions(t *testing.T) {
	t.Parallel()
	opts := object.ResolvePutOptions()
	assert.Empty(t, opts.ContentType)
	assert.Nil(t, opts.Metadata)
}

func TestResolvePutOptions_WithContentType(t *testing.T) {
	t.Parallel()
	opts := object.ResolvePutOptions(
		object.WithContentType("application/json"),
	)
	assert.Equal(t, "application/json", opts.ContentType)
}

func TestResolvePutOptions_WithMetadata(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"author": "test", "version": "1.0"}
	opts := object.ResolvePutOptions(
		object.WithMetadata(meta),
	)
	assert.Equal(t, "test", opts.Metadata["author"])
	assert.Equal(t, "1.0", opts.Metadata["version"])
}

func TestResolvePutOptions_MultipleOptions(t *testing.T) {
	t.Parallel()
	opts := object.ResolvePutOptions(
		object.WithContentType("text/plain"),
		object.WithMetadata(map[string]string{"key": "value"}),
	)
	assert.Equal(t, "text/plain", opts.ContentType)
	assert.Equal(t, "value", opts.Metadata["key"])
}

func TestResolvePutOptions_LastContentTypeWins(t *testing.T) {
	t.Parallel()
	opts := object.ResolvePutOptions(
		object.WithContentType("text/plain"),
		object.WithContentType("application/json"),
	)
	assert.Equal(t, "application/json", opts.ContentType)
}

func TestResolvePutOptions_NilMetadata(t *testing.T) {
	t.Parallel()
	opts := object.ResolvePutOptions(
		object.WithMetadata(nil),
	)
	assert.Nil(t, opts.Metadata)
}

func TestResolvePutOptions_EmptyContentType(t *testing.T) {
	t.Parallel()
	opts := object.ResolvePutOptions(
		object.WithContentType(""),
	)
	assert.Empty(t, opts.ContentType)
}

func TestResolvePutOptions_UnicodeMetadata(t *testing.T) {
	t.Parallel()
	meta := map[string]string{
		"\u4f5c\u8005": "\u5f20\u4e09",
		"description":  "\u4e2d\u6587\u63cf\u8ff0",
	}
	opts := object.ResolvePutOptions(
		object.WithMetadata(meta),
	)
	assert.Equal(t, "\u5f20\u4e09", opts.Metadata["\u4f5c\u8005"])
}
