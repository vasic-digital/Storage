package s3

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/sirupsen/logrus"

	"digital.vasic.storage/pkg/object"
)

// Client implements object.ObjectStore and object.BucketManager for
// S3-compatible storage (MinIO, AWS S3).
type Client struct {
	config      *Config
	minioClient *minio.Client
	logger      *logrus.Logger
	mu          sync.RWMutex
	connected   bool
}

// NewClient creates a new S3 client. If config is nil, DefaultConfig is used.
func NewClient(config *Config, logger *logrus.Logger) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &Client{
		config:    config,
		logger:    logger,
		connected: false,
	}, nil
}

// Connect establishes a connection to the S3-compatible endpoint.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	minioClient, err := minio.New(c.config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.config.AccessKey, c.config.SecretKey, ""),
		Secure: c.config.UseSSL,
		Region: c.config.Region,
	})
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	c.minioClient = minioClient

	// Verify connectivity
	_, err = minioClient.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to S3: %w", err)
	}

	c.connected = true
	c.logger.Info("Connected to S3-compatible storage")
	return nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	c.minioClient = nil
	return nil
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// HealthCheck verifies connectivity to the S3-compatible endpoint.
func (c *Client) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to S3")
	}

	_, err := c.minioClient.ListBuckets(ctx)
	return err
}

// CreateBucket creates a new bucket with the given configuration.
func (c *Client) CreateBucket(
	ctx context.Context,
	config object.BucketConfig,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to S3")
	}

	exists, err := c.minioClient.BucketExists(ctx, config.Name)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if exists {
		c.logger.WithField("bucket", config.Name).
			Debug("Bucket already exists")
		return nil
	}

	opts := minio.MakeBucketOptions{
		Region:        c.config.Region,
		ObjectLocking: config.ObjectLocking,
	}

	if err := c.minioClient.MakeBucket(ctx, config.Name, opts); err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	if config.Versioning {
		versionConfig := minio.BucketVersioningConfiguration{
			Status: "Enabled",
		}
		if err := c.minioClient.SetBucketVersioning(
			ctx, config.Name, versionConfig,
		); err != nil {
			return fmt.Errorf("failed to enable versioning: %w", err)
		}
	}

	if config.RetentionDays > 0 {
		rule := lifecycle.Rule{
			ID:     "auto-expire",
			Status: "Enabled",
			Expiration: lifecycle.Expiration{
				Days: lifecycle.ExpirationDays(config.RetentionDays),
			},
		}

		lcConfig := &lifecycle.Configuration{
			Rules: []lifecycle.Rule{rule},
		}

		if err := c.minioClient.SetBucketLifecycle(
			ctx, config.Name, lcConfig,
		); err != nil {
			c.logger.WithError(err).
				Warn("Failed to set lifecycle policy")
		}
	}

	c.logger.WithField("bucket", config.Name).Info("Bucket created")
	return nil
}

// DeleteBucket removes a bucket by name.
func (c *Client) DeleteBucket(ctx context.Context, bucketName string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to S3")
	}

	if err := c.minioClient.RemoveBucket(ctx, bucketName); err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	c.logger.WithField("bucket", bucketName).Info("Bucket deleted")
	return nil
}

// ListBuckets returns all available buckets.
func (c *Client) ListBuckets(
	ctx context.Context,
) ([]object.BucketInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return nil, fmt.Errorf("not connected to S3")
	}

	buckets, err := c.minioClient.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	result := make([]object.BucketInfo, len(buckets))
	for i, bucket := range buckets {
		result[i] = object.BucketInfo{
			Name:         bucket.Name,
			CreationDate: bucket.CreationDate,
		}
	}

	return result, nil
}

// BucketExists checks whether a bucket exists.
func (c *Client) BucketExists(
	ctx context.Context,
	bucketName string,
) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return false, fmt.Errorf("not connected to S3")
	}

	return c.minioClient.BucketExists(ctx, bucketName)
}

// PutObject uploads an object to the specified bucket.
func (c *Client) PutObject(
	ctx context.Context,
	bucketName string,
	objectName string,
	reader io.Reader,
	size int64,
	opts ...object.PutOption,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to S3")
	}

	resolved := object.ResolvePutOptions(opts...)

	putOpts := minio.PutObjectOptions{
		PartSize: uint64(c.config.PartSize), // #nosec G115
	}
	if resolved.ContentType != "" {
		putOpts.ContentType = resolved.ContentType
	}
	if resolved.Metadata != nil {
		putOpts.UserMetadata = resolved.Metadata
	}

	_, err := c.minioClient.PutObject(
		ctx, bucketName, objectName, reader, size, putOpts,
	)
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"object": objectName,
		"size":   size,
	}).Debug("Object uploaded")

	return nil
}

// GetObject retrieves an object from the specified bucket.
func (c *Client) GetObject(
	ctx context.Context,
	bucketName string,
	objectName string,
) (io.ReadCloser, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return nil, fmt.Errorf("not connected to S3")
	}

	obj, err := c.minioClient.GetObject(
		ctx, bucketName, objectName, minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	return obj, nil
}

// DeleteObject removes an object from the specified bucket.
func (c *Client) DeleteObject(
	ctx context.Context,
	bucketName string,
	objectName string,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to S3")
	}

	if err := c.minioClient.RemoveObject(
		ctx, bucketName, objectName, minio.RemoveObjectOptions{},
	); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"object": objectName,
	}).Debug("Object deleted")

	return nil
}

// ListObjects lists objects in a bucket matching the given prefix.
func (c *Client) ListObjects(
	ctx context.Context,
	bucketName string,
	prefix string,
) ([]object.ObjectInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return nil, fmt.Errorf("not connected to S3")
	}

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}

	var objects []object.ObjectInfo
	for obj := range c.minioClient.ListObjects(ctx, bucketName, opts) {
		if obj.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", obj.Err)
		}
		objects = append(objects, object.ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ContentType:  obj.ContentType,
			ETag:         obj.ETag,
		})
	}

	return objects, nil
}

// StatObject returns metadata about an object.
func (c *Client) StatObject(
	ctx context.Context,
	bucketName string,
	objectName string,
) (*object.ObjectInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return nil, fmt.Errorf("not connected to S3")
	}

	info, err := c.minioClient.StatObject(
		ctx, bucketName, objectName, minio.StatObjectOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}

	return &object.ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		LastModified: info.LastModified,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		Metadata:     info.UserMetadata,
	}, nil
}

// CopyObject copies an object from source to destination.
func (c *Client) CopyObject(
	ctx context.Context,
	src object.ObjectRef,
	dst object.ObjectRef,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to S3")
	}

	srcOpts := minio.CopySrcOptions{
		Bucket: src.Bucket,
		Object: src.Key,
	}

	dstOpts := minio.CopyDestOptions{
		Bucket: dst.Bucket,
		Object: dst.Key,
	}

	_, err := c.minioClient.CopyObject(ctx, dstOpts, srcOpts)
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"src_bucket": src.Bucket,
		"src_key":    src.Key,
		"dst_bucket": dst.Bucket,
		"dst_key":    dst.Key,
	}).Debug("Object copied")

	return nil
}

// GetPresignedURL generates a presigned URL for downloading an object.
func (c *Client) GetPresignedURL(
	ctx context.Context,
	bucketName string,
	objectName string,
	expiry time.Duration,
) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return "", fmt.Errorf("not connected to S3")
	}

	presignedURL, err := c.minioClient.PresignedGetObject(
		ctx, bucketName, objectName, expiry, url.Values{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.String(), nil
}

// GetPresignedPutURL generates a presigned URL for uploading an object.
func (c *Client) GetPresignedPutURL(
	ctx context.Context,
	bucketName string,
	objectName string,
	expiry time.Duration,
) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return "", fmt.Errorf("not connected to S3")
	}

	presignedURL, err := c.minioClient.PresignedPutObject(
		ctx, bucketName, objectName, expiry,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.String(), nil
}

// SetLifecycleRule sets a lifecycle rule on a bucket.
func (c *Client) SetLifecycleRule(
	ctx context.Context,
	bucketName string,
	rule *LifecycleRule,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to S3")
	}

	status := "Enabled"
	if !rule.Enabled {
		status = "Disabled"
	}

	lcRule := lifecycle.Rule{
		ID:     rule.ID,
		Status: status,
	}

	if rule.Prefix != "" {
		lcRule.RuleFilter = lifecycle.Filter{
			Prefix: rule.Prefix,
		}
	}

	if rule.ExpirationDays > 0 {
		lcRule.Expiration = lifecycle.Expiration{
			Days: lifecycle.ExpirationDays(rule.ExpirationDays),
		}
	}

	if rule.NoncurrentDays > 0 {
		lcRule.NoncurrentVersionExpiration = lifecycle.NoncurrentVersionExpiration{
			NoncurrentDays: lifecycle.ExpirationDays(rule.NoncurrentDays),
		}
	}

	if rule.DeleteMarkerExpiry {
		lcRule.Expiration.DeleteMarker = lifecycle.ExpireDeleteMarker(true)
	}

	existingConfig, err := c.minioClient.GetBucketLifecycle(ctx, bucketName)
	if err != nil {
		existingConfig = &lifecycle.Configuration{}
	}

	found := false
	for i, r := range existingConfig.Rules {
		if r.ID == rule.ID {
			existingConfig.Rules[i] = lcRule
			found = true
			break
		}
	}
	if !found {
		existingConfig.Rules = append(existingConfig.Rules, lcRule)
	}

	if err := c.minioClient.SetBucketLifecycle(
		ctx, bucketName, existingConfig,
	); err != nil {
		return fmt.Errorf("failed to set lifecycle rule: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"rule":   rule.ID,
	}).Info("Lifecycle rule set")

	return nil
}

// RemoveLifecycleRule removes a lifecycle rule from a bucket.
func (c *Client) RemoveLifecycleRule(
	ctx context.Context,
	bucketName string,
	ruleID string,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to S3")
	}

	existingConfig, err := c.minioClient.GetBucketLifecycle(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to get lifecycle config: %w", err)
	}

	var newRules []lifecycle.Rule
	for _, r := range existingConfig.Rules {
		if r.ID != ruleID {
			newRules = append(newRules, r)
		}
	}

	existingConfig.Rules = newRules

	if len(newRules) == 0 {
		if err := c.minioClient.SetBucketLifecycle(
			ctx, bucketName, nil,
		); err != nil {
			return fmt.Errorf(
				"failed to remove lifecycle config: %w", err,
			)
		}
	} else {
		if err := c.minioClient.SetBucketLifecycle(
			ctx, bucketName, existingConfig,
		); err != nil {
			return fmt.Errorf(
				"failed to update lifecycle config: %w", err,
			)
		}
	}

	c.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"rule":   ruleID,
	}).Info("Lifecycle rule removed")

	return nil
}

// Compile-time interface compliance checks.
var (
	_ object.ObjectStore   = (*Client)(nil)
	_ object.BucketManager = (*Client)(nil)
)
