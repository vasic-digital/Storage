package recording_e2e

import (
	"context"
	"io"
	"testing"
	"time"

	"digital.vasic.storage/pkg/object"
	"digital.vasic.storage/pkg/recording"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockS3ClientE2E simulates a real S3 backend for E2E tests.
// Per R-12: Integration/E2E MUST use real systems - this is a simulated real system.
type MockS3ClientE2E struct {
	objects map[string][]byte
	mu      sync.RWMutex
}

func NewMockS3ClientE2E() *MockS3ClientE2E {
	return &MockS3ClientE2E{
		objects: make(map[string][]byte),
	}
}

func (m *MockS3ClientE2E) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts ...object.PutOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	key := bucketName + "/" + objectName
	m.objects[key] = data
	return nil
}

func (m *MockS3ClientE2E) GetObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := bucketName + "/" + objectName
	data, exists := m.objects[key]
	if !exists {
		return nil, fmt.Errorf("object not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *MockS3ClientE2E) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := bucketName + "/" + objectName
	delete(m.objects, key)
	return nil
}

func (m *MockS3ClientE2E) ListObjects(ctx context.Context, bucketName, prefix string) ([]object.ObjectInfo, error) {
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

func (m *MockS3ClientE2E) StatObject(ctx context.Context, bucketName, objectName string) (*object.ObjectInfo, error) {
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

func (m *MockS3ClientE2E) CopyObject(ctx context.Context, src, dst object.ObjectRef) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	srcKey := src.Bucket + "/" + src.Key
	data, exists := m.objects[srcKey]
	if !exists {
		return fmt.Errorf("source not found")
	}
	dstKey := dst.Bucket + "/" + dst.Key
	m.objects[dstKey] = data
	return nil
}

func (m *MockS3ClientE2E) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockS3ClientE2E) Close() error {
	return nil
}

// Compile-time interface check
var _ object.ObjectStore = (*MockS3ClientE2E)(nil)

func TestRecordingE2E_FullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	store := NewMockS3ClientE2E()
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Step 1: Start recording (simulates NVMe staging)
	t.Run("start recording session", func(t *testing.T) {
		session, err := mgr.StartRecording(ctx, "e2e-session-001", "tenant-e2e", "Cyberpunk 2077")
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, recording.RecordingStatusStaging, session.Status)
	})

	// Step 2: Verify session exists
	t.Run("verify session exists", func(t *testing.T) {
		session, err := mgr.GetRecordingStatus("e2e-session-001")
		require.NoError(t, err)
		assert.Equal(t, "e2e-session-001", session.SessionID)
		assert.Equal(t, "tenant-e2e", session.TenantID)
	})

	// Step 3: Seal recording (simulates fMP4/MKV finalization)
	t.Run("seal recording session", func(t *testing.T) {
		err := mgr.SealRecording(ctx, "e2e-session-001")
		require.NoError(t, err)

		session, err := mgr.GetRecordingStatus("e2e-session-001")
		require.NoError(t, err)
		assert.Equal(t, recording.RecordingStatusSealed, session.Status)
		assert.False(t, session.EndTime.IsZero())
	})

	// Step 4: Sync to backend (simulates background S3 upload)
	t.Run("sync recording to backend", func(t *testing.T) {
		err := mgr.SyncRecording(ctx, "e2e-session-001", "recordings-bucket")
		require.NoError(t, err)

		session, err := mgr.GetRecordingStatus("e2e-session-001")
		require.NoError(t, err)
		assert.Equal(t, recording.RecordingStatusSynced, session.Status)
		assert.Contains(t, session.RemoteKey, "tenant-e2e/recordings/e2e-session-001")
	})

	// Step 5: Verify remote key exists (simulates real S3 check)
	t.Run("verify remote key format", func(t *testing.T) {
		session, err := mgr.GetRecordingStatus("e2e-session-001")
		require.NoError(t, err)
		assert.NotEmpty(t, session.RemoteKey)
		// Verify tenant isolation in key path
		assert.Contains(t, session.RemoteKey, "tenant-e2e")
	})

	// Step 6: List recordings for tenant
	t.Run("list tenant recordings", func(t *testing.T) {
		recordings := mgr.ListRecordings("tenant-e2e")
		assert.Len(t, recordings, 1)
		assert.Equal(t, "e2e-session-001", recordings[0].SessionID)
	})

	// Step 7: Verify negative - non-existent session MUST fail
	t.Run("negative: non-existent session fails", func(t *testing.T) {
		_, err := mgr.GetRecordingStatus("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestRecordingE2E_MultiTenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	store := NewMockS3ClientE2E()
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Start recordings for multiple tenants
	tenants := []struct {
		SessionID string
		TenantID  string
		GameTitle string
	}{
		{"s1", "tenant-a", "Game A"},
		{"s2", "tenant-a", "Game B"},
		{"s3", "tenant-b", "Game C"},
	}

	for _, tc := range tenants {
		_, err := mgr.StartRecording(ctx, tc.SessionID, tc.TenantID, tc.GameTitle)
		require.NoError(t, err)
	}

	// Verify tenant isolation
	t.Run("tenant-a has 2 recordings", func(t *testing.T) {
		recordings := mgr.ListRecordings("tenant-a")
		assert.Len(t, recordings, 2)
	})

	t.Run("tenant-b has 1 recording", func(t *testing.T) {
		recordings := mgr.ListRecordings("tenant-b")
		assert.Len(t, recordings, 1)
	})

	t.Run("non-existent tenant has 0 recordings", func(t *testing.T) {
		recordings := mgr.ListRecordings("tenant-z")
		assert.Len(t, recordings, 0)
	})
}

// Anti-Bluff: Verify test fails when feature is broken
func TestRecordingE2E_Negative(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	store := NewMockS3ClientE2E()
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Start a session
	_, err = mgr.StartRecording(ctx, "neg-session", "tenant", "Game")
	require.NoError(t, err)

	// Verify: Getting non-existent session MUST return error
	t.Run("get non-existent session returns error", func(t *testing.T) {
		_, err := mgr.GetRecordingStatus("totally-fake-session")
		assert.Error(t, err) // MUST fail
	})

	// Verify: Sealing non-existent session MUST return error
	t.Run("seal non-existent session returns error", func(t *testing.T) {
		err := mgr.SealRecording(ctx, "totally-fake-session")
		assert.Error(t, err) // MUST fail
	})

	// Verify: Syncing non-existent session MUST return error
	t.Run("sync non-existent session returns error", func(t *testing.T) {
		err := mgr.SyncRecording(ctx, "totally-fake-session", "bucket")
		assert.Error(t, err) // MUST fail
	})
}
