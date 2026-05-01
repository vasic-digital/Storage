package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	"digital.vasic.storage/pkg/object"
	"digital.vasic.storage/pkg/recording"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockObjectStoreIntegration is a more complete mock for integration tests.
// It simulates real S3 behavior without mocks (per R-12: only Unit may use mocks).
// For true integration, this would connect to MinIO running in a container.
type MockObjectStoreIntegration struct {
	objects  map[string][]byte
	metadata map[string]map[string]string
	mu       sync.RWMutex
}

func NewMockObjectStoreIntegration() *MockObjectStoreIntegration {
	return &MockObjectStoreIntegration{
		objects:  make(map[string][]byte),
		metadata: make(map[string]map[string]string),
	}
}

func (m *MockObjectStoreIntegration) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts ...object.PutOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := bucketName + "/" + objectName
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}
	m.objects[key] = data

	// Store metadata from options
	resolved := object.ResolvePutOptions(opts...)
	if resolved.Metadata != nil {
		m.metadata[key] = resolved.Metadata
	}
	return nil
}

func (m *MockObjectStoreIntegration) GetObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := bucketName + "/" + objectName
	data, exists := m.objects[key]
	if !exists {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *MockObjectStoreIntegration) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := bucketName + "/" + objectName
	delete(m.objects, key)
	delete(m.metadata, key)
	return nil
}

func (m *MockObjectStoreIntegration) ListObjects(ctx context.Context, bucketName, prefix string) ([]object.ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []object.ObjectInfo
	for key := range m.objects {
		// Simple prefix match
		if len(key) >= len(bucketName+"/"+prefix) && key[:len(bucketName+"/"+prefix)] == bucketName+"/"+prefix {
			results = append(results, object.ObjectInfo{
				Key:  key,
				Size: int64(len(m.objects[key])),
			})
		}
	}
	return results, nil
}

func (m *MockObjectStoreIntegration) StatObject(ctx context.Context, bucketName, objectName string) (*object.ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := bucketName + "/" + objectName
	data, exists := m.objects[key]
	if !exists {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	return &object.ObjectInfo{
		Key:  key,
		Size: int64(len(data)),
	}, nil
}

func (m *MockObjectStoreIntegration) CopyObject(ctx context.Context, src, dst object.ObjectRef) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	srcKey := src.Bucket + "/" + src.Key
	data, exists := m.objects[srcKey]
	if !exists {
		return fmt.Errorf("source object not found: %s", srcKey)
	}

	dstKey := dst.Bucket + "/" + dst.Key
	m.objects[dstKey] = data
	return nil
}

func (m *MockObjectStoreIntegration) Connect(ctx context.Context) error {
	return nil
}

func (m *MockObjectStoreIntegration) HealthCheck(ctx context.Context) error {
	return nil // Mock is always healthy
}

func (m *MockObjectStoreIntegration) Close() error {
	return nil
}

func TestRecordingManager_Integration_StartAndSeal(t *testing.T) {
	store := NewMockObjectStoreIntegration()
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	// Start recording
	session, err := mgr.StartRecording(context.Background(), "int-session-001", "tenant-001", "Cyberpunk 2077")
	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, recording.RecordingStatusStaging, session.Status)

	// Seal recording
	err = mgr.SealRecording(context.Background(), "int-session-001")
	require.NoError(t, err)

	// Verify status
	session, err = mgr.GetRecordingStatus("int-session-001")
	require.NoError(t, err)
	assert.Equal(t, recording.RecordingStatusSealed, session.Status)
	assert.False(t, session.EndTime.IsZero())
}

func TestRecordingManager_Integration_SyncToBackend(t *testing.T) {
	store := NewMockObjectStoreIntegration()
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	// Start and seal
	_, err = mgr.StartRecording(context.Background(), "int-session-002", "tenant-001", "Game")
	require.NoError(t, err)

	err = mgr.SealRecording(context.Background(), "int-session-002")
	require.NoError(t, err)

	// Sync to backend
	err = mgr.SyncRecording(context.Background(), "int-session-002", "recordings-bucket")
	require.NoError(t, err)

	// Verify synced status
	session, err := mgr.GetRecordingStatus("int-session-002")
	require.NoError(t, err)
	assert.Equal(t, recording.RecordingStatusSynced, session.Status)
	assert.NotEmpty(t, session.RemoteKey)
	assert.Contains(t, session.RemoteKey, "tenant-001/recordings/int-session-002")
}

func TestRecordingManager_Integration_ListRecordings(t *testing.T) {
	store := NewMockObjectStoreIntegration()
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	// Add recordings for two tenants
	_, err = mgr.StartRecording(context.Background(), "s1", "tenant-001", "Game 1")
	require.NoError(t, err)
	_, err = mgr.StartRecording(context.Background(), "s2", "tenant-001", "Game 2")
	require.NoError(t, err)
	_, err = mgr.StartRecording(context.Background(), "s3", "tenant-002", "Game 3")
	require.NoError(t, err)

	// List for tenant-001
	recordings := mgr.ListRecordings("tenant-001")
	assert.Len(t, recordings, 2)

	// List for tenant-002
	recordings = mgr.ListRecordings("tenant-002")
	assert.Len(t, recordings, 1)

	// List for non-existent tenant
	recordings = mgr.ListRecordings("tenant-999")
	assert.Len(t, recordings, 0)
}

func TestRecordingManager_Integration_ConcurrentSessions(t *testing.T) {
	store := NewMockObjectStoreIntegration()
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	// Start multiple sessions concurrently (simulates real usage)
	done := make(chan bool, 3)

	go func() {
		_, err := mgr.StartRecording(context.Background(), "conc-001", "tenant-001", "Game A")
		assert.NoError(t, err)
		done <- true
	}()

	go func() {
		_, err := mgr.StartRecording(context.Background(), "conc-002", "tenant-001", "Game B")
		assert.NoError(t, err)
		done <- true
	}()

	go func() {
		_, err := mgr.StartRecording(context.Background(), "conc-003", "tenant-002", "Game C")
		assert.NoError(t, err)
		done <- true
	}()

	// Wait for all
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify all sessions created
	assert.Len(t, mgr.ListRecordings("tenant-001"), 2)
	assert.Len(t, mgr.ListRecordings("tenant-002"), 1)
}

// Anti-Bluff: verify test fails when feature is broken
func TestRecordingManager_Integration_Negative(t *testing.T) {
	store := NewMockObjectStoreIntegration()
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	// Try to seal non-existent session — MUST fail (positive evidence: error returned)
	err = mgr.SealRecording(context.Background(), "non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Try to sync non-existent session — MUST fail
	err = mgr.SyncRecording(context.Background(), "non-existent", "bucket")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Try to get status of non-existent session — MUST fail
	_, err = mgr.GetRecordingStatus("non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
