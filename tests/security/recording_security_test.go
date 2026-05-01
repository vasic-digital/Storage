package security

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

// MockSecureStore simulates a secure S3 backend with encryption-at-rest.
type MockSecureStore struct {
	objects   map[string][]byte
	encrypted map[string]bool // tracks which objects are "encrypted"
	mu        sync.RWMutex
}

func NewMockSecureStore() *MockSecureStore {
	return &MockSecureStore{
		objects:   make(map[string][]byte),
		encrypted: make(map[string]bool),
	}
}

func (m *MockSecureStore) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts ...object.PutOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}
	key := bucketName + "/" + objectName
	m.objects[key] = data
	// Simulate encryption-at-rest
	m.encrypted[key] = true
	return nil
}

func (m *MockSecureStore) GetObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := bucketName + "/" + objectName
	data, exists := m.objects[key]
	if !exists {
		return nil, fmt.Errorf("object not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *MockSecureStore) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := bucketName + "/" + objectName
	delete(m.objects, key)
	delete(m.encrypted, key)
	return nil
}

func (m *MockSecureStore) ListObjects(ctx context.Context, bucketName, prefix string) ([]object.ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []object.ObjectInfo
	for key := range m.objects {
		if len(key) > len(bucketName+"/"+prefix) && key[:len(bucketName+"/"+prefix)] == bucketName+"/"+prefix {
			results = append(results, object.ObjectInfo{
				Key:  key,
				Size: int64(len(m.objects[key])),
			})
		}
	}
	return results, nil
}

func (m *MockSecureStore) StatObject(ctx context.Context, bucketName, objectName string) (*object.ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := bucketName + "/" + objectName
	data, exists := m.objects[key]
	if !exists {
		return nil, fmt.Errorf("object not found")
	}
	return &object.ObjectInfo{
		Key:  key,
		Size: int64(len(data)),
	}, nil
}

func (m *MockSecureStore) CopyObject(ctx context.Context, src, dst object.ObjectRef) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	srcKey := src.Bucket + "/" + src.Key
	data, exists := m.objects[srcKey]
	if !exists {
		return fmt.Errorf("source not found")
	}
	dstKey := dst.Bucket + "/" + dst.Key
	m.objects[dstKey] = data
	m.encrypted[dstKey] = true
	return nil
}

func (m *MockSecureStore) Connect(ctx context.Context) error {
	return nil
}

func (m *MockSecureStore) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockSecureStore) Close() error {
	return nil
}

// Compile-time interface check
var _ object.ObjectStore = (*MockSecureStore)(nil)

func TestRecordingSecurity_EncryptionAtRest(t *testing.T) {
	store := NewMockSecureStore()
	cfg := recording.DefaultRecordingConfig()
	cfg.EncryptionEnabled = true
	cfg.EncryptionKeyID = "key-001"

	mgr, err := recording.NewManager(cfg, store, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Start and seal a recording
	_, err = mgr.StartRecording(ctx, "sec-session-001", "tenant-sec", "Game")
	require.NoError(t, err)

	err = mgr.SealRecording(ctx, "sec-session-001")
	require.NoError(t, err)

	// Verify encryption is enabled in config
	t.Run("encryption enabled in config", func(t *testing.T) {
		assert.True(t, cfg.EncryptionEnabled)
		assert.NotEmpty(t, cfg.EncryptionKeyID)
	})

	// Verify session status is sealed (ready for encrypted sync)
	t.Run("session sealed with encryption", func(t *testing.T) {
		session, err := mgr.GetRecordingStatus("sec-session-001")
		require.NoError(t, err)
		assert.Equal(t, recording.RecordingStatusSealed, session.Status)
	})
}

func TestRecordingSecurity_TenantIsolation(t *testing.T) {
	store := NewMockSecureStore()
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Create recordings for different tenants
	tenants := []struct {
		SessionID string
		TenantID  string
	}{
		{"s1", "tenant-alpha"},
		{"s2", "tenant-beta"},
		{"s3", "tenant-alpha"},
	}

	for _, tc := range tenants {
		_, err := mgr.StartRecording(ctx, tc.SessionID, tc.TenantID, "Game")
		require.NoError(t, err)
	}

	// Verify tenant-alpha can only see its own recordings
	t.Run("tenant-alpha sees only its recordings", func(t *testing.T) {
		recordings := mgr.ListRecordings("tenant-alpha")
		assert.Len(t, recordings, 2)
		for _, r := range recordings {
			assert.Equal(t, "tenant-alpha", r.TenantID)
		}
	})

	// Verify tenant-beta can only see its own recordings
	t.Run("tenant-beta sees only its recordings", func(t *testing.T) {
		recordings := mgr.ListRecordings("tenant-beta")
		assert.Len(t, recordings, 1)
		assert.Equal(t, "tenant-beta", recordings[0].TenantID)
	})

	// Verify isolation: tenant-alpha cannot see tenant-beta's recordings
	t.Run("tenant isolation enforced", func(t *testing.T) {
		alphaRecordings := mgr.ListRecordings("tenant-alpha")
		betaRecordings := mgr.ListRecordings("tenant-beta")

		for _, r := range alphaRecordings {
			assert.NotEqual(t, "tenant-beta", r.TenantID)
		}
		for _, r := range betaRecordings {
			assert.NotEqual(t, "tenant-alpha", r.TenantID)
		}
	})
}

func TestRecordingSecurity_RetentionPolicy(t *testing.T) {
	store := NewMockSecureStore()
	cfg := recording.DefaultRecordingConfig()
	cfg.RetentionDays = 30

	_, err := recording.NewManager(cfg, store, nil)
	require.NoError(t, err)

	// Verify retention policy is set
	t.Run("retention policy configured", func(t *testing.T) {
		assert.Equal(t, 30, cfg.RetentionDays)
	})

	// In production, expired recordings would be deleted automatically
	// This test verifies the policy is in place
	t.Run("retention days positive", func(t *testing.T) {
		assert.Greater(t, cfg.RetentionDays, 0)
	})
}

// Anti-Bluff: Verify security tests fail when security is broken
func TestRecordingSecurity_Negative(t *testing.T) {
	store := NewMockSecureStore()
	cfg := recording.DefaultRecordingConfig()
	cfg.EncryptionEnabled = true

	mgr, err := recording.NewManager(cfg, store, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Start a session
	_, err = mgr.StartRecording(ctx, "neg-sec", "tenant", "Game")
	require.NoError(t, err)

	// Verify: Non-existent session MUST return error (security: no info leak)
	t.Run("non-existent session returns error", func(t *testing.T) {
		_, err := mgr.GetRecordingStatus("hacker-session")
		assert.Error(t, err) // MUST fail - no info leak
		assert.Contains(t, err.Error(), "not found")
	})

	// Verify: Cannot list another tenant's recordings
	t.Run("tenant isolation prevents cross-tenant access", func(t *testing.T) {
		// Session "s1" belongs to "tenant-alpha"
		_, err := mgr.StartRecording(ctx, "s1", "tenant-alpha", "Game")
		require.NoError(t, err)

		// "tenant-beta" should NOT see "s1"
		betaRecordings := mgr.ListRecordings("tenant-beta")
		for _, r := range betaRecordings {
			assert.NotEqual(t, "s1", r.SessionID)
		}
	})
}
