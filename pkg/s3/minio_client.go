package s3

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
)

// MinioClient defines the interface for MinIO client operations.
// This interface allows for easy mocking in tests.
type MinioClient interface {
	ListBuckets(ctx context.Context) ([]minio.BucketInfo, error)
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	RemoveBucket(ctx context.Context, bucketName string) error
	SetBucketVersioning(
		ctx context.Context, bucketName string, config minio.BucketVersioningConfiguration,
	) error
	SetBucketLifecycle(ctx context.Context, bucketName string, config *lifecycle.Configuration) error
	GetBucketLifecycle(ctx context.Context, bucketName string) (*lifecycle.Configuration, error)
	PutObject(
		ctx context.Context, bucketName, objectName string,
		reader io.Reader, objectSize int64, opts minio.PutObjectOptions,
	) (minio.UploadInfo, error)
	GetObject(
		ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions,
	) (*minio.Object, error)
	RemoveObject(
		ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions,
	) error
	ListObjects(
		ctx context.Context, bucketName string, opts minio.ListObjectsOptions,
	) <-chan minio.ObjectInfo
	StatObject(
		ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions,
	) (minio.ObjectInfo, error)
	CopyObject(
		ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions,
	) (minio.UploadInfo, error)
	PresignedGetObject(
		ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values,
	) (*url.URL, error)
	PresignedPutObject(
		ctx context.Context, bucketName, objectName string, expires time.Duration,
	) (*url.URL, error)
}

// Verify that *minio.Client implements MinioClient.
var _ MinioClient = (*minio.Client)(nil)
