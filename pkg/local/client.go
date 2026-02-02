package local

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"digital.vasic.storage/pkg/object"
)

// metaSuffix is appended to object paths to store sidecar metadata.
const metaSuffix = ".meta"

// Client implements object.ObjectStore and object.BucketManager using
// the local filesystem. Buckets map to directories and objects map to files.
type Client struct {
	rootDir   string
	logger    *logrus.Logger
	mu        sync.RWMutex
	connected bool
}

// Config holds configuration for the local filesystem client.
type Config struct {
	RootDir string `json:"root_dir" yaml:"root_dir"`
}

// NewClient creates a new local filesystem client. The rootDir must be an
// absolute or resolvable path where buckets (directories) will be stored.
func NewClient(config *Config, logger *logrus.Logger) (*Client, error) {
	if config == nil || config.RootDir == "" {
		return nil, fmt.Errorf("root_dir is required")
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &Client{
		rootDir:   config.RootDir,
		logger:    logger,
		connected: false,
	}, nil
}

// Connect ensures the root directory exists and marks the client as connected.
func (c *Client) Connect(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(c.rootDir, 0o755); err != nil {
		return fmt.Errorf("failed to create root directory: %w", err)
	}

	c.connected = true
	c.logger.WithField("root", c.rootDir).
		Info("Connected to local filesystem storage")
	return nil
}

// Close marks the client as disconnected.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// HealthCheck verifies the root directory is accessible.
func (c *Client) HealthCheck(_ context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to local storage")
	}

	_, err := os.Stat(c.rootDir)
	if err != nil {
		return fmt.Errorf("root directory inaccessible: %w", err)
	}
	return nil
}

// CreateBucket creates a directory for the bucket.
func (c *Client) CreateBucket(
	_ context.Context,
	config object.BucketConfig,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to local storage")
	}

	bucketPath := filepath.Join(c.rootDir, config.Name)
	if err := os.MkdirAll(bucketPath, 0o755); err != nil {
		return fmt.Errorf("failed to create bucket directory: %w", err)
	}

	c.logger.WithField("bucket", config.Name).Info("Bucket created")
	return nil
}

// DeleteBucket removes the bucket directory. The bucket must be empty.
func (c *Client) DeleteBucket(_ context.Context, name string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to local storage")
	}

	bucketPath := filepath.Join(c.rootDir, name)
	if err := os.Remove(bucketPath); err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	c.logger.WithField("bucket", name).Info("Bucket deleted")
	return nil
}

// ListBuckets returns all bucket directories.
func (c *Client) ListBuckets(
	_ context.Context,
) ([]object.BucketInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to local storage")
	}

	entries, err := os.ReadDir(c.rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	var buckets []object.BucketInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		buckets = append(buckets, object.BucketInfo{
			Name:         entry.Name(),
			CreationDate: info.ModTime(),
		})
	}

	return buckets, nil
}

// BucketExists checks whether a bucket directory exists.
func (c *Client) BucketExists(_ context.Context, name string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return false, fmt.Errorf("not connected to local storage")
	}

	bucketPath := filepath.Join(c.rootDir, name)
	info, err := os.Stat(bucketPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check bucket: %w", err)
	}

	return info.IsDir(), nil
}

// PutObject writes an object as a file inside the bucket directory.
func (c *Client) PutObject(
	_ context.Context,
	bucket string,
	key string,
	reader io.Reader,
	_ int64,
	opts ...object.PutOption,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to local storage")
	}

	objPath := filepath.Join(c.rootDir, bucket, key)

	if err := os.MkdirAll(filepath.Dir(objPath), 0o755); err != nil {
		return fmt.Errorf("failed to create object directory: %w", err)
	}

	f, err := os.Create(objPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("failed to write object: %w", err)
	}

	// Store metadata sidecar if options are provided
	resolved := object.ResolvePutOptions(opts...)
	if resolved.ContentType != "" || resolved.Metadata != nil {
		meta := sidecarMeta{
			ContentType: resolved.ContentType,
			Metadata:    resolved.Metadata,
		}
		if err := writeSidecar(objPath, &meta); err != nil {
			c.logger.WithError(err).
				Warn("Failed to write metadata sidecar")
		}
	}

	c.logger.WithFields(logrus.Fields{
		"bucket": bucket,
		"key":    key,
	}).Debug("Object uploaded")

	return nil
}

// GetObject opens the file for reading.
func (c *Client) GetObject(
	_ context.Context,
	bucket string,
	key string,
) (io.ReadCloser, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to local storage")
	}

	objPath := filepath.Join(c.rootDir, bucket, key)
	f, err := os.Open(objPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open object: %w", err)
	}

	return f, nil
}

// DeleteObject removes the file and its sidecar metadata.
func (c *Client) DeleteObject(
	_ context.Context,
	bucket string,
	key string,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to local storage")
	}

	objPath := filepath.Join(c.rootDir, bucket, key)
	if err := os.Remove(objPath); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	// Remove sidecar metadata if present
	_ = os.Remove(objPath + metaSuffix)

	c.logger.WithFields(logrus.Fields{
		"bucket": bucket,
		"key":    key,
	}).Debug("Object deleted")

	return nil
}

// ListObjects walks the bucket directory and returns matching objects.
func (c *Client) ListObjects(
	_ context.Context,
	bucket string,
	prefix string,
) ([]object.ObjectInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to local storage")
	}

	bucketPath := filepath.Join(c.rootDir, bucket)
	var objects []object.ObjectInfo

	err := filepath.Walk(bucketPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Skip sidecar metadata files
		if strings.HasSuffix(path, metaSuffix) {
			return nil
		}

		relPath, err := filepath.Rel(bucketPath, path)
		if err != nil {
			return err
		}
		// Normalize to forward slashes
		relPath = filepath.ToSlash(relPath)

		if prefix != "" && !strings.HasPrefix(relPath, prefix) {
			return nil
		}

		objInfo := object.ObjectInfo{
			Key:          relPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
		}

		// Load sidecar metadata if present
		meta, err := readSidecar(path)
		if err == nil && meta != nil {
			objInfo.ContentType = meta.ContentType
			objInfo.Metadata = meta.Metadata
		}

		objects = append(objects, objInfo)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	return objects, nil
}

// StatObject returns metadata about an object file.
func (c *Client) StatObject(
	_ context.Context,
	bucket string,
	key string,
) (*object.ObjectInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to local storage")
	}

	objPath := filepath.Join(c.rootDir, bucket, key)
	info, err := os.Stat(objPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}

	objInfo := &object.ObjectInfo{
		Key:          key,
		Size:         info.Size(),
		LastModified: info.ModTime(),
	}

	meta, err := readSidecar(objPath)
	if err == nil && meta != nil {
		objInfo.ContentType = meta.ContentType
		objInfo.Metadata = meta.Metadata
	}

	return objInfo, nil
}

// CopyObject copies a file from source to destination.
func (c *Client) CopyObject(
	_ context.Context,
	src object.ObjectRef,
	dst object.ObjectRef,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to local storage")
	}

	srcPath := filepath.Join(c.rootDir, src.Bucket, src.Key)
	dstPath := filepath.Join(c.rootDir, dst.Bucket, dst.Key)

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source object: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination object: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	// Copy sidecar metadata if present
	srcMeta := srcPath + metaSuffix
	if _, err := os.Stat(srcMeta); err == nil {
		metaSrc, err := os.Open(srcMeta)
		if err == nil {
			defer func() { _ = metaSrc.Close() }()
			metaDst, err := os.Create(dstPath + metaSuffix)
			if err == nil {
				defer func() { _ = metaDst.Close() }()
				_, _ = io.Copy(metaDst, metaSrc)
			}
		}
	}

	c.logger.WithFields(logrus.Fields{
		"src_bucket": src.Bucket,
		"src_key":    src.Key,
		"dst_bucket": dst.Bucket,
		"dst_key":    dst.Key,
	}).Debug("Object copied")

	return nil
}

// sidecarMeta holds metadata stored in a .meta JSON sidecar file.
type sidecarMeta struct {
	ContentType string            `json:"content_type,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

func writeSidecar(objPath string, meta *sidecarMeta) error {
	meta.CreatedAt = time.Now()
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return os.WriteFile(objPath+metaSuffix, data, 0o644)
}

func readSidecar(objPath string) (*sidecarMeta, error) {
	data, err := os.ReadFile(objPath + metaSuffix)
	if err != nil {
		return nil, err
	}
	var meta sidecarMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	return &meta, nil
}

// Compile-time interface compliance checks.
var (
	_ object.ObjectStore   = (*Client)(nil)
	_ object.BucketManager = (*Client)(nil)
)
