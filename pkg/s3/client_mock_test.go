package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.storage/pkg/object"
)

// mockMinioClient implements MinioClient for testing.
type mockMinioClient struct {
	listBucketsFunc         func(ctx context.Context) ([]minio.BucketInfo, error)
	bucketExistsFunc        func(ctx context.Context, bucketName string) (bool, error)
	makeBucketFunc          func(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	removeBucketFunc        func(ctx context.Context, bucketName string) error
	setBucketVersioningFunc func(ctx context.Context, bucketName string, config minio.BucketVersioningConfiguration) error
	setBucketLifecycleFunc  func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error
	getBucketLifecycleFunc  func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error)
	putObjectFunc           func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	getObjectFunc           func(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	removeObjectFunc        func(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	listObjectsFunc         func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	statObjectFunc          func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	copyObjectFunc          func(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error)
	presignedGetObjectFunc  func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
	presignedPutObjectFunc  func(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error)
}

func (m *mockMinioClient) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	if m.listBucketsFunc != nil {
		return m.listBucketsFunc(ctx)
	}
	return nil, nil
}

func (m *mockMinioClient) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	if m.bucketExistsFunc != nil {
		return m.bucketExistsFunc(ctx, bucketName)
	}
	return false, nil
}

func (m *mockMinioClient) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	if m.makeBucketFunc != nil {
		return m.makeBucketFunc(ctx, bucketName, opts)
	}
	return nil
}

func (m *mockMinioClient) RemoveBucket(ctx context.Context, bucketName string) error {
	if m.removeBucketFunc != nil {
		return m.removeBucketFunc(ctx, bucketName)
	}
	return nil
}

func (m *mockMinioClient) SetBucketVersioning(ctx context.Context, bucketName string, config minio.BucketVersioningConfiguration) error {
	if m.setBucketVersioningFunc != nil {
		return m.setBucketVersioningFunc(ctx, bucketName, config)
	}
	return nil
}

func (m *mockMinioClient) SetBucketLifecycle(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
	if m.setBucketLifecycleFunc != nil {
		return m.setBucketLifecycleFunc(ctx, bucketName, config)
	}
	return nil
}

func (m *mockMinioClient) GetBucketLifecycle(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
	if m.getBucketLifecycleFunc != nil {
		return m.getBucketLifecycleFunc(ctx, bucketName)
	}
	return &lifecycle.Configuration{}, nil
}

func (m *mockMinioClient) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	if m.putObjectFunc != nil {
		return m.putObjectFunc(ctx, bucketName, objectName, reader, objectSize, opts)
	}
	return minio.UploadInfo{}, nil
}

func (m *mockMinioClient) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	if m.getObjectFunc != nil {
		return m.getObjectFunc(ctx, bucketName, objectName, opts)
	}
	return nil, nil
}

func (m *mockMinioClient) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	if m.removeObjectFunc != nil {
		return m.removeObjectFunc(ctx, bucketName, objectName, opts)
	}
	return nil
}

func (m *mockMinioClient) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	if m.listObjectsFunc != nil {
		return m.listObjectsFunc(ctx, bucketName, opts)
	}
	ch := make(chan minio.ObjectInfo)
	close(ch)
	return ch
}

func (m *mockMinioClient) StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	if m.statObjectFunc != nil {
		return m.statObjectFunc(ctx, bucketName, objectName, opts)
	}
	return minio.ObjectInfo{}, nil
}

func (m *mockMinioClient) CopyObject(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
	if m.copyObjectFunc != nil {
		return m.copyObjectFunc(ctx, dst, src)
	}
	return minio.UploadInfo{}, nil
}

func (m *mockMinioClient) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	if m.presignedGetObjectFunc != nil {
		return m.presignedGetObjectFunc(ctx, bucketName, objectName, expires, reqParams)
	}
	return &url.URL{Scheme: "https", Host: "example.com", Path: "/test"}, nil
}

func (m *mockMinioClient) PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error) {
	if m.presignedPutObjectFunc != nil {
		return m.presignedPutObjectFunc(ctx, bucketName, objectName, expires)
	}
	return &url.URL{Scheme: "https", Host: "example.com", Path: "/test"}, nil
}

// createClientWithMock creates a client with a mock for testing.
func createClientWithMock(mock *mockMinioClient) *Client {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress logs during tests

	client, _ := NewClient(nil, logger)
	client.SetMinioClientForTest(mock)

	return client
}

// S3 Error types for testing.
var (
	errAccessDenied       = errors.New("Access Denied")
	errNoSuchBucket       = errors.New("The specified bucket does not exist")
	errNoSuchKey          = errors.New("The specified key does not exist")
	errBucketNotEmpty     = errors.New("The bucket you tried to delete is not empty")
	errBucketAlreadyOwned = errors.New("Your previous request to create the named bucket succeeded")
	errPermissionDenied   = errors.New("Access Denied: Permission denied")
	errInvalidAccessKey   = errors.New("The Access Key Id you provided does not exist")
	errSignatureError     = errors.New("The request signature we calculated does not match")
	errNetworkError       = errors.New("dial tcp: connection refused")
	errTimeout            = errors.New("context deadline exceeded")
	errRateLimited        = errors.New("SlowDown: Please reduce your request rate")
	errServiceUnavailable = errors.New("Service Unavailable")
	errInternalError      = errors.New("InternalError: We encountered an internal error")
	errMultipartUpload    = errors.New("Error completing multipart upload")
	errInvalidObjectName  = errors.New("Object name contains invalid characters")
	errEntityTooLarge     = errors.New("EntityTooLarge: Your proposed upload exceeds the maximum")
)

// ============================================================================
// Test Error Scenarios for CreateBucket
// ============================================================================

func TestClient_CreateBucket_BucketExistsCheckError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "access denied checking existence",
			err:       errAccessDenied,
			expectMsg: "failed to check bucket existence",
		},
		{
			name:      "network error checking existence",
			err:       errNetworkError,
			expectMsg: "failed to check bucket existence",
		},
		{
			name:      "timeout checking existence",
			err:       errTimeout,
			expectMsg: "failed to check bucket existence",
		},
		{
			name:      "invalid credentials",
			err:       errInvalidAccessKey,
			expectMsg: "failed to check bucket existence",
		},
		{
			name:      "signature mismatch",
			err:       errSignatureError,
			expectMsg: "failed to check bucket existence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
					return false, tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()
			config := object.BucketConfig{Name: "test-bucket"}

			err := client.CreateBucket(ctx, config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_CreateBucket_MakeBucketError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "access denied creating bucket",
			err:       errAccessDenied,
			expectMsg: "failed to create bucket",
		},
		{
			name:      "bucket already exists owned by you",
			err:       errBucketAlreadyOwned,
			expectMsg: "failed to create bucket",
		},
		{
			name:      "permission denied",
			err:       errPermissionDenied,
			expectMsg: "failed to create bucket",
		},
		{
			name:      "service unavailable",
			err:       errServiceUnavailable,
			expectMsg: "failed to create bucket",
		},
		{
			name:      "internal error",
			err:       errInternalError,
			expectMsg: "failed to create bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
					return false, nil // Bucket doesn't exist
				},
				makeBucketFunc: func(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
					return tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()
			config := object.BucketConfig{Name: "test-bucket"}

			err := client.CreateBucket(ctx, config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_CreateBucket_SetVersioningError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "access denied setting versioning",
			err:       errAccessDenied,
			expectMsg: "failed to enable versioning",
		},
		{
			name:      "permission denied",
			err:       errPermissionDenied,
			expectMsg: "failed to enable versioning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
					return false, nil
				},
				makeBucketFunc: func(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
					return nil
				},
				setBucketVersioningFunc: func(ctx context.Context, bucketName string, config minio.BucketVersioningConfiguration) error {
					return tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()
			config := object.BucketConfig{Name: "test-bucket", Versioning: true}

			err := client.CreateBucket(ctx, config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_CreateBucket_BucketAlreadyExists(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
			return true, nil // Bucket already exists
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	config := object.BucketConfig{Name: "existing-bucket"}

	err := client.CreateBucket(ctx, config)
	require.NoError(t, err) // Should succeed without error
}

func TestClient_CreateBucket_Success(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
			return false, nil
		},
		makeBucketFunc: func(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	config := object.BucketConfig{Name: "new-bucket"}

	err := client.CreateBucket(ctx, config)
	require.NoError(t, err)
}

func TestClient_CreateBucket_WithVersioning(t *testing.T) {
	versioningSet := false
	mock := &mockMinioClient{
		bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
			return false, nil
		},
		makeBucketFunc: func(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
			return nil
		},
		setBucketVersioningFunc: func(ctx context.Context, bucketName string, config minio.BucketVersioningConfiguration) error {
			versioningSet = true
			assert.Equal(t, "Enabled", config.Status)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	config := object.BucketConfig{Name: "versioned-bucket", Versioning: true}

	err := client.CreateBucket(ctx, config)
	require.NoError(t, err)
	assert.True(t, versioningSet)
}

func TestClient_CreateBucket_WithRetention(t *testing.T) {
	lifecycleSet := false
	mock := &mockMinioClient{
		bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
			return false, nil
		},
		makeBucketFunc: func(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
			return nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			lifecycleSet = true
			require.Len(t, config.Rules, 1)
			assert.Equal(t, "auto-expire", config.Rules[0].ID)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	config := object.BucketConfig{Name: "retention-bucket", RetentionDays: 30}

	err := client.CreateBucket(ctx, config)
	require.NoError(t, err)
	assert.True(t, lifecycleSet)
}

func TestClient_CreateBucket_RetentionFailureLogsWarning(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
			return false, nil
		},
		makeBucketFunc: func(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
			return nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			return errors.New("lifecycle error") // Error is logged, not returned
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	config := object.BucketConfig{Name: "retention-bucket", RetentionDays: 30}

	// Should succeed even though lifecycle failed (only logs warning)
	err := client.CreateBucket(ctx, config)
	require.NoError(t, err)
}

func TestClient_CreateBucket_WithObjectLocking(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
			return false, nil
		},
		makeBucketFunc: func(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
			assert.True(t, opts.ObjectLocking)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	config := object.BucketConfig{Name: "locked-bucket", ObjectLocking: true}

	err := client.CreateBucket(ctx, config)
	require.NoError(t, err)
}

// ============================================================================
// Test Error Scenarios for DeleteBucket
// ============================================================================

func TestClient_DeleteBucket_Errors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "bucket not empty",
			err:       errBucketNotEmpty,
			expectMsg: "failed to delete bucket",
		},
		{
			name:      "bucket not found",
			err:       errNoSuchBucket,
			expectMsg: "failed to delete bucket",
		},
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "failed to delete bucket",
		},
		{
			name:      "permission denied",
			err:       errPermissionDenied,
			expectMsg: "failed to delete bucket",
		},
		{
			name:      "network error",
			err:       errNetworkError,
			expectMsg: "failed to delete bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				removeBucketFunc: func(ctx context.Context, bucketName string) error {
					return tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			err := client.DeleteBucket(ctx, "test-bucket")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_DeleteBucket_Success(t *testing.T) {
	mock := &mockMinioClient{
		removeBucketFunc: func(ctx context.Context, bucketName string) error {
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	err := client.DeleteBucket(ctx, "test-bucket")
	require.NoError(t, err)
}

// ============================================================================
// Test Error Scenarios for ListBuckets
// ============================================================================

func TestClient_ListBuckets_Errors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "failed to list buckets",
		},
		{
			name:      "invalid credentials",
			err:       errInvalidAccessKey,
			expectMsg: "failed to list buckets",
		},
		{
			name:      "signature error",
			err:       errSignatureError,
			expectMsg: "failed to list buckets",
		},
		{
			name:      "network error",
			err:       errNetworkError,
			expectMsg: "failed to list buckets",
		},
		{
			name:      "service unavailable",
			err:       errServiceUnavailable,
			expectMsg: "failed to list buckets",
		},
		{
			name:      "rate limited",
			err:       errRateLimited,
			expectMsg: "failed to list buckets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				listBucketsFunc: func(ctx context.Context) ([]minio.BucketInfo, error) {
					return nil, tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			buckets, err := client.ListBuckets(ctx)
			require.Error(t, err)
			assert.Nil(t, buckets)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_ListBuckets_Success(t *testing.T) {
	now := time.Now()
	mock := &mockMinioClient{
		listBucketsFunc: func(ctx context.Context) ([]minio.BucketInfo, error) {
			return []minio.BucketInfo{
				{Name: "bucket1", CreationDate: now},
				{Name: "bucket2", CreationDate: now.Add(-24 * time.Hour)},
				{Name: "bucket3", CreationDate: now.Add(-48 * time.Hour)},
			}, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	buckets, err := client.ListBuckets(ctx)
	require.NoError(t, err)
	assert.Len(t, buckets, 3)
	assert.Equal(t, "bucket1", buckets[0].Name)
	assert.Equal(t, "bucket2", buckets[1].Name)
	assert.Equal(t, "bucket3", buckets[2].Name)
}

func TestClient_ListBuckets_Empty(t *testing.T) {
	mock := &mockMinioClient{
		listBucketsFunc: func(ctx context.Context) ([]minio.BucketInfo, error) {
			return []minio.BucketInfo{}, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	buckets, err := client.ListBuckets(ctx)
	require.NoError(t, err)
	assert.Empty(t, buckets)
}

// ============================================================================
// Test Error Scenarios for BucketExists
// ============================================================================

func TestClient_BucketExists_Success(t *testing.T) {
	tests := []struct {
		name   string
		exists bool
	}{
		{"bucket exists", true},
		{"bucket does not exist", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
					return tt.exists, nil
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			exists, err := client.BucketExists(ctx, "test-bucket")
			require.NoError(t, err)
			assert.Equal(t, tt.exists, exists)
		})
	}
}

func TestClient_BucketExists_Error(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsFunc: func(ctx context.Context, bucketName string) (bool, error) {
			return false, errAccessDenied
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	exists, err := client.BucketExists(ctx, "test-bucket")
	require.Error(t, err)
	assert.False(t, exists)
}

// ============================================================================
// Test Error Scenarios for PutObject (Upload Errors)
// ============================================================================

func TestClient_PutObject_Errors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "failed to upload object",
		},
		{
			name:      "permission denied",
			err:       errPermissionDenied,
			expectMsg: "failed to upload object",
		},
		{
			name:      "bucket not found",
			err:       errNoSuchBucket,
			expectMsg: "failed to upload object",
		},
		{
			name:      "entity too large",
			err:       errEntityTooLarge,
			expectMsg: "failed to upload object",
		},
		{
			name:      "multipart upload failure",
			err:       errMultipartUpload,
			expectMsg: "failed to upload object",
		},
		{
			name:      "invalid object name",
			err:       errInvalidObjectName,
			expectMsg: "failed to upload object",
		},
		{
			name:      "network error",
			err:       errNetworkError,
			expectMsg: "failed to upload object",
		},
		{
			name:      "timeout",
			err:       errTimeout,
			expectMsg: "failed to upload object",
		},
		{
			name:      "rate limited",
			err:       errRateLimited,
			expectMsg: "failed to upload object",
		},
		{
			name:      "internal error",
			err:       errInternalError,
			expectMsg: "failed to upload object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				putObjectFunc: func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
					return minio.UploadInfo{}, tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			err := client.PutObject(ctx, "bucket", "key", bytes.NewReader([]byte("data")), 4)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_PutObject_Success(t *testing.T) {
	mock := &mockMinioClient{
		putObjectFunc: func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{Bucket: bucketName, Key: objectName, Size: objectSize}, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	err := client.PutObject(ctx, "bucket", "key", bytes.NewReader([]byte("data")), 4)
	require.NoError(t, err)
}

func TestClient_PutObject_WithContentType(t *testing.T) {
	var capturedOpts minio.PutObjectOptions
	mock := &mockMinioClient{
		putObjectFunc: func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			capturedOpts = opts
			return minio.UploadInfo{}, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	err := client.PutObject(ctx, "bucket", "key", bytes.NewReader([]byte("data")), 4,
		object.WithContentType("application/json"))
	require.NoError(t, err)
	assert.Equal(t, "application/json", capturedOpts.ContentType)
}

func TestClient_PutObject_WithMetadata(t *testing.T) {
	var capturedOpts minio.PutObjectOptions
	mock := &mockMinioClient{
		putObjectFunc: func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			capturedOpts = opts
			return minio.UploadInfo{}, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	metadata := map[string]string{"author": "test", "version": "1.0"}
	err := client.PutObject(ctx, "bucket", "key", bytes.NewReader([]byte("data")), 4,
		object.WithMetadata(metadata))
	require.NoError(t, err)
	assert.Equal(t, metadata, capturedOpts.UserMetadata)
}

// ============================================================================
// Test Error Scenarios for GetObject (Download Errors)
// ============================================================================

func TestClient_GetObject_Errors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "object not found",
			err:       errNoSuchKey,
			expectMsg: "failed to get object",
		},
		{
			name:      "bucket not found",
			err:       errNoSuchBucket,
			expectMsg: "failed to get object",
		},
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "failed to get object",
		},
		{
			name:      "permission denied",
			err:       errPermissionDenied,
			expectMsg: "failed to get object",
		},
		{
			name:      "network error",
			err:       errNetworkError,
			expectMsg: "failed to get object",
		},
		{
			name:      "timeout",
			err:       errTimeout,
			expectMsg: "failed to get object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				getObjectFunc: func(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
					return nil, tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			obj, err := client.GetObject(ctx, "bucket", "key")
			require.Error(t, err)
			assert.Nil(t, obj)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_GetObject_Success(t *testing.T) {
	mock := &mockMinioClient{
		getObjectFunc: func(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
			// Return nil object (in real implementation this would be a valid object)
			return nil, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	obj, err := client.GetObject(ctx, "bucket", "key")
	require.NoError(t, err)
	// obj can be nil in our mock; in real implementation it would be a valid object
	_ = obj
}

// ============================================================================
// Test Error Scenarios for DeleteObject
// ============================================================================

func TestClient_DeleteObject_Errors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "object not found",
			err:       errNoSuchKey,
			expectMsg: "failed to delete object",
		},
		{
			name:      "bucket not found",
			err:       errNoSuchBucket,
			expectMsg: "failed to delete object",
		},
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "failed to delete object",
		},
		{
			name:      "permission denied",
			err:       errPermissionDenied,
			expectMsg: "failed to delete object",
		},
		{
			name:      "network error",
			err:       errNetworkError,
			expectMsg: "failed to delete object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				removeObjectFunc: func(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
					return tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			err := client.DeleteObject(ctx, "bucket", "key")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_DeleteObject_Success(t *testing.T) {
	mock := &mockMinioClient{
		removeObjectFunc: func(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	err := client.DeleteObject(ctx, "bucket", "key")
	require.NoError(t, err)
}

// ============================================================================
// Test Error Scenarios for ListObjects (Pagination Errors)
// ============================================================================

func TestClient_ListObjects_Errors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "bucket not found",
			err:       errNoSuchBucket,
			expectMsg: "error listing objects",
		},
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "error listing objects",
		},
		{
			name:      "permission denied",
			err:       errPermissionDenied,
			expectMsg: "error listing objects",
		},
		{
			name:      "network error",
			err:       errNetworkError,
			expectMsg: "error listing objects",
		},
		{
			name:      "timeout",
			err:       errTimeout,
			expectMsg: "error listing objects",
		},
		{
			name:      "internal error during pagination",
			err:       errInternalError,
			expectMsg: "error listing objects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
					ch := make(chan minio.ObjectInfo, 1)
					go func() {
						ch <- minio.ObjectInfo{Err: tt.err}
						close(ch)
					}()
					return ch
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			objs, err := client.ListObjects(ctx, "bucket", "")
			require.Error(t, err)
			assert.Nil(t, objs)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_ListObjects_PartialResultsWithError(t *testing.T) {
	mock := &mockMinioClient{
		listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			ch := make(chan minio.ObjectInfo, 3)
			go func() {
				ch <- minio.ObjectInfo{Key: "object1", Size: 100}
				ch <- minio.ObjectInfo{Key: "object2", Size: 200}
				ch <- minio.ObjectInfo{Err: errInternalError}
				close(ch)
			}()
			return ch
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	objs, err := client.ListObjects(ctx, "bucket", "")
	require.Error(t, err)
	assert.Nil(t, objs)
	assert.Contains(t, err.Error(), "error listing objects")
}

func TestClient_ListObjects_Success(t *testing.T) {
	now := time.Now()
	mock := &mockMinioClient{
		listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			ch := make(chan minio.ObjectInfo, 3)
			go func() {
				ch <- minio.ObjectInfo{Key: "file1.txt", Size: 100, LastModified: now, ContentType: "text/plain", ETag: "abc123"}
				ch <- minio.ObjectInfo{Key: "file2.txt", Size: 200, LastModified: now}
				ch <- minio.ObjectInfo{Key: "dir/file3.txt", Size: 300, LastModified: now}
				close(ch)
			}()
			return ch
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	objs, err := client.ListObjects(ctx, "bucket", "")
	require.NoError(t, err)
	assert.Len(t, objs, 3)
	assert.Equal(t, "file1.txt", objs[0].Key)
	assert.Equal(t, int64(100), objs[0].Size)
	assert.Equal(t, "text/plain", objs[0].ContentType)
	assert.Equal(t, "abc123", objs[0].ETag)
	assert.Equal(t, "file2.txt", objs[1].Key)
	assert.Equal(t, "dir/file3.txt", objs[2].Key)
}

func TestClient_ListObjects_Empty(t *testing.T) {
	mock := &mockMinioClient{
		listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			ch := make(chan minio.ObjectInfo)
			close(ch)
			return ch
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	objs, err := client.ListObjects(ctx, "bucket", "")
	require.NoError(t, err)
	assert.Empty(t, objs)
}

func TestClient_ListObjects_WithPrefix(t *testing.T) {
	var capturedOpts minio.ListObjectsOptions
	mock := &mockMinioClient{
		listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			capturedOpts = opts
			ch := make(chan minio.ObjectInfo)
			close(ch)
			return ch
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	_, err := client.ListObjects(ctx, "bucket", "logs/2024/")
	require.NoError(t, err)
	assert.Equal(t, "logs/2024/", capturedOpts.Prefix)
	assert.True(t, capturedOpts.Recursive)
}

// ============================================================================
// Test Error Scenarios for StatObject
// ============================================================================

func TestClient_StatObject_Errors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "object not found",
			err:       errNoSuchKey,
			expectMsg: "failed to stat object",
		},
		{
			name:      "bucket not found",
			err:       errNoSuchBucket,
			expectMsg: "failed to stat object",
		},
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "failed to stat object",
		},
		{
			name:      "permission denied",
			err:       errPermissionDenied,
			expectMsg: "failed to stat object",
		},
		{
			name:      "network error",
			err:       errNetworkError,
			expectMsg: "failed to stat object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				statObjectFunc: func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
					return minio.ObjectInfo{}, tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			info, err := client.StatObject(ctx, "bucket", "key")
			require.Error(t, err)
			assert.Nil(t, info)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_StatObject_Success(t *testing.T) {
	now := time.Now()
	mock := &mockMinioClient{
		statObjectFunc: func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{
				Key:          objectName,
				Size:         1024,
				LastModified: now,
				ContentType:  "application/json",
				ETag:         "abc123",
				UserMetadata: map[string]string{"author": "test"},
			}, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	info, err := client.StatObject(ctx, "bucket", "key")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "key", info.Key)
	assert.Equal(t, int64(1024), info.Size)
	assert.Equal(t, "application/json", info.ContentType)
	assert.Equal(t, "abc123", info.ETag)
	assert.Equal(t, "test", info.Metadata["author"])
}

// ============================================================================
// Test Error Scenarios for CopyObject
// ============================================================================

func TestClient_CopyObject_Errors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "source not found",
			err:       errNoSuchKey,
			expectMsg: "failed to copy object",
		},
		{
			name:      "source bucket not found",
			err:       errNoSuchBucket,
			expectMsg: "failed to copy object",
		},
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "failed to copy object",
		},
		{
			name:      "permission denied",
			err:       errPermissionDenied,
			expectMsg: "failed to copy object",
		},
		{
			name:      "network error",
			err:       errNetworkError,
			expectMsg: "failed to copy object",
		},
		{
			name:      "entity too large",
			err:       errEntityTooLarge,
			expectMsg: "failed to copy object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				copyObjectFunc: func(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
					return minio.UploadInfo{}, tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()
			src := object.ObjectRef{Bucket: "src-bucket", Key: "src-key"}
			dst := object.ObjectRef{Bucket: "dst-bucket", Key: "dst-key"}

			err := client.CopyObject(ctx, src, dst)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_CopyObject_Success(t *testing.T) {
	var capturedSrc minio.CopySrcOptions
	var capturedDst minio.CopyDestOptions
	mock := &mockMinioClient{
		copyObjectFunc: func(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
			capturedSrc = src
			capturedDst = dst
			return minio.UploadInfo{}, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	src := object.ObjectRef{Bucket: "src-bucket", Key: "src-key"}
	dst := object.ObjectRef{Bucket: "dst-bucket", Key: "dst-key"}

	err := client.CopyObject(ctx, src, dst)
	require.NoError(t, err)
	assert.Equal(t, "src-bucket", capturedSrc.Bucket)
	assert.Equal(t, "src-key", capturedSrc.Object)
	assert.Equal(t, "dst-bucket", capturedDst.Bucket)
	assert.Equal(t, "dst-key", capturedDst.Object)
}

// ============================================================================
// Test Error Scenarios for Presigned URLs
// ============================================================================

func TestClient_GetPresignedURL_Errors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "bucket not found",
			err:       errNoSuchBucket,
			expectMsg: "failed to generate presigned URL",
		},
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "failed to generate presigned URL",
		},
		{
			name:      "invalid credentials",
			err:       errInvalidAccessKey,
			expectMsg: "failed to generate presigned URL",
		},
		{
			name:      "network error",
			err:       errNetworkError,
			expectMsg: "failed to generate presigned URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				presignedGetObjectFunc: func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
					return nil, tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			urlStr, err := client.GetPresignedURL(ctx, "bucket", "key", time.Hour)
			require.Error(t, err)
			assert.Empty(t, urlStr)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_GetPresignedURL_Success(t *testing.T) {
	expectedURL := &url.URL{
		Scheme:   "https",
		Host:     "s3.example.com",
		Path:     "/bucket/key",
		RawQuery: "X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Expires=3600",
	}

	mock := &mockMinioClient{
		presignedGetObjectFunc: func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
			return expectedURL, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	urlStr, err := client.GetPresignedURL(ctx, "bucket", "key", time.Hour)
	require.NoError(t, err)
	assert.Equal(t, expectedURL.String(), urlStr)
}

func TestClient_GetPresignedPutURL_Errors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "bucket not found",
			err:       errNoSuchBucket,
			expectMsg: "failed to generate presigned URL",
		},
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "failed to generate presigned URL",
		},
		{
			name:      "invalid credentials",
			err:       errInvalidAccessKey,
			expectMsg: "failed to generate presigned URL",
		},
		{
			name:      "network error",
			err:       errNetworkError,
			expectMsg: "failed to generate presigned URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				presignedPutObjectFunc: func(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error) {
					return nil, tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			urlStr, err := client.GetPresignedPutURL(ctx, "bucket", "key", time.Hour)
			require.Error(t, err)
			assert.Empty(t, urlStr)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_GetPresignedPutURL_Success(t *testing.T) {
	expectedURL := &url.URL{
		Scheme:   "https",
		Host:     "s3.example.com",
		Path:     "/bucket/key",
		RawQuery: "X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Expires=3600",
	}

	mock := &mockMinioClient{
		presignedPutObjectFunc: func(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error) {
			return expectedURL, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	urlStr, err := client.GetPresignedPutURL(ctx, "bucket", "key", time.Hour)
	require.NoError(t, err)
	assert.Equal(t, expectedURL.String(), urlStr)
}

// ============================================================================
// Test Error Scenarios for SetLifecycleRule
// ============================================================================

func TestClient_SetLifecycleRule_GetExistingError(t *testing.T) {
	// Test when GetBucketLifecycle fails but we continue with empty config
	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return nil, errors.New("lifecycle configuration does not exist")
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	rule := &LifecycleRule{
		ID:             "test-rule",
		Enabled:        true,
		ExpirationDays: 30,
	}

	// This should not error - the code handles missing lifecycle gracefully
	err := client.SetLifecycleRule(ctx, "bucket", rule)
	assert.NoError(t, err)
}

func TestClient_SetLifecycleRule_SetError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectMsg string
	}{
		{
			name:      "access denied",
			err:       errAccessDenied,
			expectMsg: "failed to set lifecycle rule",
		},
		{
			name:      "permission denied",
			err:       errPermissionDenied,
			expectMsg: "failed to set lifecycle rule",
		},
		{
			name:      "bucket not found",
			err:       errNoSuchBucket,
			expectMsg: "failed to set lifecycle rule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
					return &lifecycle.Configuration{}, nil
				},
				setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
					return tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()
			rule := &LifecycleRule{
				ID:             "test-rule",
				Enabled:        true,
				ExpirationDays: 30,
			}

			err := client.SetLifecycleRule(ctx, "bucket", rule)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectMsg)
		})
	}
}

func TestClient_SetLifecycleRule_UpdateExistingRule(t *testing.T) {
	existingRules := []lifecycle.Rule{
		{ID: "existing-rule", Status: "Enabled"},
	}

	updateCalled := false
	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{Rules: existingRules}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			updateCalled = true
			// Verify the rule was updated, not appended
			require.Len(t, config.Rules, 1)
			assert.Equal(t, "existing-rule", config.Rules[0].ID)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	rule := &LifecycleRule{
		ID:             "existing-rule",
		Enabled:        true,
		ExpirationDays: 60,
	}

	err := client.SetLifecycleRule(ctx, "bucket", rule)
	require.NoError(t, err)
	assert.True(t, updateCalled)
}

func TestClient_SetLifecycleRule_AddNewRule(t *testing.T) {
	existingRules := []lifecycle.Rule{
		{ID: "existing-rule", Status: "Enabled"},
	}

	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{Rules: existingRules}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			// Verify the new rule was appended
			require.Len(t, config.Rules, 2)
			assert.Equal(t, "existing-rule", config.Rules[0].ID)
			assert.Equal(t, "new-rule", config.Rules[1].ID)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	rule := &LifecycleRule{
		ID:             "new-rule",
		Enabled:        true,
		ExpirationDays: 30,
	}

	err := client.SetLifecycleRule(ctx, "bucket", rule)
	require.NoError(t, err)
}

func TestClient_SetLifecycleRule_DisabledRule(t *testing.T) {
	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			require.Len(t, config.Rules, 1)
			assert.Equal(t, "Disabled", config.Rules[0].Status)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	rule := &LifecycleRule{
		ID:             "disabled-rule",
		Enabled:        false,
		ExpirationDays: 30,
	}

	err := client.SetLifecycleRule(ctx, "bucket", rule)
	require.NoError(t, err)
}

func TestClient_SetLifecycleRule_WithPrefix(t *testing.T) {
	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			require.Len(t, config.Rules, 1)
			assert.Equal(t, "logs/", config.Rules[0].RuleFilter.Prefix)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	rule := &LifecycleRule{
		ID:             "prefix-rule",
		Prefix:         "logs/",
		Enabled:        true,
		ExpirationDays: 30,
	}

	err := client.SetLifecycleRule(ctx, "bucket", rule)
	require.NoError(t, err)
}

func TestClient_SetLifecycleRule_WithNoncurrentDays(t *testing.T) {
	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			require.Len(t, config.Rules, 1)
			assert.Equal(t, lifecycle.ExpirationDays(30), config.Rules[0].NoncurrentVersionExpiration.NoncurrentDays)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	rule := &LifecycleRule{
		ID:             "noncurrent-rule",
		Enabled:        true,
		NoncurrentDays: 30,
	}

	err := client.SetLifecycleRule(ctx, "bucket", rule)
	require.NoError(t, err)
}

func TestClient_SetLifecycleRule_WithDeleteMarkerExpiry(t *testing.T) {
	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			require.Len(t, config.Rules, 1)
			assert.True(t, bool(config.Rules[0].Expiration.DeleteMarker))
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	rule := &LifecycleRule{
		ID:                 "marker-rule",
		Enabled:            true,
		DeleteMarkerExpiry: true,
	}

	err := client.SetLifecycleRule(ctx, "bucket", rule)
	require.NoError(t, err)
}

func TestClient_SetLifecycleRule_WithExpirationDays(t *testing.T) {
	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			require.Len(t, config.Rules, 1)
			assert.Equal(t, lifecycle.ExpirationDays(90), config.Rules[0].Expiration.Days)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()
	rule := &LifecycleRule{
		ID:             "expiration-rule",
		Enabled:        true,
		ExpirationDays: 90,
	}

	err := client.SetLifecycleRule(ctx, "bucket", rule)
	require.NoError(t, err)
}

// ============================================================================
// Test Error Scenarios for RemoveLifecycleRule
// ============================================================================

func TestClient_RemoveLifecycleRule_GetError(t *testing.T) {
	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return nil, errAccessDenied
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	err := client.RemoveLifecycleRule(ctx, "bucket", "rule-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get lifecycle config")
}

func TestClient_RemoveLifecycleRule_SetError_LastRule(t *testing.T) {
	existingRules := []lifecycle.Rule{
		{ID: "rule-to-remove", Status: "Enabled"},
	}

	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{Rules: existingRules}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			return errAccessDenied
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	err := client.RemoveLifecycleRule(ctx, "bucket", "rule-to-remove")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove lifecycle config")
}

func TestClient_RemoveLifecycleRule_SetError_OtherRulesRemain(t *testing.T) {
	existingRules := []lifecycle.Rule{
		{ID: "rule-to-remove", Status: "Enabled"},
		{ID: "other-rule", Status: "Enabled"},
	}

	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{Rules: existingRules}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			return errAccessDenied
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	err := client.RemoveLifecycleRule(ctx, "bucket", "rule-to-remove")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update lifecycle config")
}

func TestClient_RemoveLifecycleRule_Success_LastRule(t *testing.T) {
	existingRules := []lifecycle.Rule{
		{ID: "rule-to-remove", Status: "Enabled"},
	}

	setBucketLifecycleCalled := false
	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{Rules: existingRules}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			setBucketLifecycleCalled = true
			// When removing the last rule, config should be nil
			assert.Nil(t, config)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	err := client.RemoveLifecycleRule(ctx, "bucket", "rule-to-remove")
	require.NoError(t, err)
	assert.True(t, setBucketLifecycleCalled)
}

func TestClient_RemoveLifecycleRule_Success_OtherRulesRemain(t *testing.T) {
	existingRules := []lifecycle.Rule{
		{ID: "rule-to-remove", Status: "Enabled"},
		{ID: "other-rule", Status: "Enabled"},
	}

	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{Rules: existingRules}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			require.NotNil(t, config)
			require.Len(t, config.Rules, 1)
			assert.Equal(t, "other-rule", config.Rules[0].ID)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	err := client.RemoveLifecycleRule(ctx, "bucket", "rule-to-remove")
	require.NoError(t, err)
}

func TestClient_RemoveLifecycleRule_RuleNotFound(t *testing.T) {
	existingRules := []lifecycle.Rule{
		{ID: "other-rule", Status: "Enabled"},
	}

	mock := &mockMinioClient{
		getBucketLifecycleFunc: func(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
			return &lifecycle.Configuration{Rules: existingRules}, nil
		},
		setBucketLifecycleFunc: func(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
			// Rule was not found, but the original rules are passed through
			require.Len(t, config.Rules, 1)
			assert.Equal(t, "other-rule", config.Rules[0].ID)
			return nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	// Attempting to remove a non-existent rule should still succeed
	err := client.RemoveLifecycleRule(ctx, "bucket", "non-existent-rule")
	require.NoError(t, err)
}

// ============================================================================
// Test Error Scenarios for HealthCheck
// ============================================================================

func TestClient_HealthCheck_Errors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "access denied",
			err:  errAccessDenied,
		},
		{
			name: "network error",
			err:  errNetworkError,
		},
		{
			name: "timeout",
			err:  errTimeout,
		},
		{
			name: "service unavailable",
			err:  errServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinioClient{
				listBucketsFunc: func(ctx context.Context) ([]minio.BucketInfo, error) {
					return nil, tt.err
				},
			}

			client := createClientWithMock(mock)
			ctx := context.Background()

			err := client.HealthCheck(ctx)
			require.Error(t, err)
			assert.Equal(t, tt.err, err)
		})
	}
}

func TestClient_HealthCheck_Success(t *testing.T) {
	mock := &mockMinioClient{
		listBucketsFunc: func(ctx context.Context) ([]minio.BucketInfo, error) {
			return []minio.BucketInfo{}, nil
		},
	}

	client := createClientWithMock(mock)
	ctx := context.Background()

	err := client.HealthCheck(ctx)
	require.NoError(t, err)
}

// ============================================================================
// Additional Edge Case Tests
// ============================================================================

func TestClient_SetMinioClientForTest(t *testing.T) {
	client, err := NewClient(nil, nil)
	require.NoError(t, err)
	assert.False(t, client.IsConnected())

	mock := &mockMinioClient{}
	client.SetMinioClientForTest(mock)

	assert.True(t, client.IsConnected())
}
