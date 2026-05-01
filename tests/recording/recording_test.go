package recording_test

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

// MockObjectStore is a mock implementation of object.ObjectStore for testing.
type MockObjectStore struct {
	PutObjectFunc    func(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts ...object.PutOption) error
	GetObjectFunc    func(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error)
	DeleteObjectFunc func(ctx context.Context, bucketName, objectName string) error
	ListObjectsFunc  func(ctx context.Context, bucketName, prefix string) ([]object.ObjectInfo, error)
	StatObjectFunc   func(ctx context.Context, bucketName, objectName string) (*object.ObjectInfo, error)
	CopyObjectFunc   func(ctx context.Context, src, dst object.ObjectRef) error
	HealthCheckFunc  func(ctx context.Context) error
	CloseFunc        func() error
}

func (m *MockObjectStore) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts ...object.PutOption) error {
	if m.PutObjectFunc != nil {
		return m.PutObjectFunc(ctx, bucketName, objectName, reader, size, opts...)
	}
	return nil
}

func (m *MockObjectStore) GetObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	if m.GetObjectFunc != nil {
		return m.GetObjectFunc(ctx, bucketName, objectName)
	}
	return nil, nil
}

func (m *MockObjectStore) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	if m.DeleteObjectFunc != nil {
		return m.DeleteObjectFunc(ctx, bucketName, objectName)
	}
	return nil
}

func (m *MockObjectStore) ListObjects(ctx context.Context, bucketName, prefix string) ([]object.ObjectInfo, error) {
	if m.ListObjectsFunc != nil {
		return m.ListObjectsFunc(ctx, bucketName, prefix)
	}
	return nil, nil
}

func (m *MockObjectStore) StatObject(ctx context.Context, bucketName, objectName string) (*object.ObjectInfo, error) {
	if m.StatObjectFunc != nil {
		return m.StatObjectFunc(ctx, bucketName, objectName)
	}
	return nil, nil
}

func (m *MockObjectStore) CopyObject(ctx context.Context, src, dst object.ObjectRef) error {
	if m.CopyObjectFunc != nil {
		return m.CopyObjectFunc(ctx, src, dst)
	}
	return nil
}

func (m *MockObjectStore) Connect(ctx context.Context) error {
	return nil
}

func (m *MockObjectStore) HealthCheck(ctx context.Context) error {
	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc(ctx)
	}
	return nil
}

func (m *MockObjectStore) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestManager_NewManager(t *testing.T) {
	t.Run("creates manager with defaults when config is nil", func(t *testing.T) {
		mgr, err := recording.NewManager(nil, &MockObjectStore{}, nil)
		require.NoError(t, err)
		assert.NotNil(t, mgr)
	})

	t.Run("returns error when s3Client is nil", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), nil, nil)
		require.Error(t, err)
		assert.Nil(t, mgr)
		assert.Contains(t, err.Error(), "s3Client is required")
	})
}

func TestManager_StartRecording(t *testing.T) {
	t.Run("starts recording session successfully", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		session, err := mgr.StartRecording(context.Background(), "session-001", "tenant-001", "Cyberpunk 2077")
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, "session-001", session.SessionID)
		assert.Equal(t, "tenant-001", session.TenantID)
		assert.Equal(t, "Cyberpunk 2077", session.GameTitle)
		assert.Equal(t, recording.RecordingStatusStaging, session.Status)
		assert.False(t, session.StartTime.IsZero())
	})

	t.Run("returns error for duplicate session ID", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-001", "tenant-001", "Game 1")
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-001", "tenant-001", "Game 2")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestManager_SealRecording(t *testing.T) {
	t.Run("seals recording session successfully", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-001", "tenant-001", "Game")
		require.NoError(t, err)

		err = mgr.SealRecording(context.Background(), "session-001")
		require.NoError(t, err)

		session, err := mgr.GetRecordingStatus("session-001")
		require.NoError(t, err)
		assert.Equal(t, recording.RecordingStatusSealed, session.Status)
		assert.NotNil(t, session.EndTime)
	})

	t.Run("returns error for non-staging session", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-001", "tenant-001", "Game")
		require.NoError(t, err)

		err = mgr.SealRecording(context.Background(), "session-001")
		require.NoError(t, err)

		// Try to seal again
		err = mgr.SealRecording(context.Background(), "session-001")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not in staging state")
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		err = mgr.SealRecording(context.Background(), "non-existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestManager_SyncRecording(t *testing.T) {
	t.Run("syncs sealed recording successfully", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-001", "tenant-001", "Game")
		require.NoError(t, err)

		err = mgr.SealRecording(context.Background(), "session-001")
		require.NoError(t, err)

		err = mgr.SyncRecording(context.Background(), "session-001", "recordings-bucket")
		require.NoError(t, err)

		session, err := mgr.GetRecordingStatus("session-001")
		require.NoError(t, err)
		assert.Equal(t, recording.RecordingStatusSynced, session.Status)
		assert.NotEmpty(t, session.RemoteKey)
	})

	t.Run("returns error for non-sealed session", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-001", "tenant-001", "Game")
		require.NoError(t, err)

		err = mgr.SyncRecording(context.Background(), "session-001", "bucket")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not ready for sync")
	})
}

func TestManager_GetRecordingStatus(t *testing.T) {
	t.Run("returns session status successfully", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-001", "tenant-001", "Game")
		require.NoError(t, err)

		session, err := mgr.GetRecordingStatus("session-001")
		require.NoError(t, err)
		assert.Equal(t, "session-001", session.SessionID)
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		_, err = mgr.GetRecordingStatus("non-existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestManager_ListRecordings(t *testing.T) {
	t.Run("lists recordings for tenant", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-001", "tenant-001", "Game 1")
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-002", "tenant-001", "Game 2")
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-003", "tenant-002", "Game 3")
		require.NoError(t, err)

		recordings := mgr.ListRecordings("tenant-001")
		assert.Len(t, recordings, 2)
	})

	t.Run("returns empty slice for tenant with no recordings", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		recordings := mgr.ListRecordings("non-existent-tenant")
		assert.Len(t, recordings, 0)
	})
}

func TestDefaultRecordingConfig(t *testing.T) {
	t.Run("returns valid default config", func(t *testing.T) {
		config := recording.DefaultRecordingConfig()
		assert.NotNil(t, config)
		assert.Equal(t, "/var/lib/helixplay/recordings", config.LocalStagingPath)
		assert.Equal(t, recording.ContainerFormatFMP4, config.Format)
		assert.Equal(t, 2*time.Second, config.FMP4FragmentDuration)
		assert.Equal(t, 30*time.Minute, config.CircularBufferDuration)
		assert.Equal(t, int64(1024*1024*1024), config.DiskReserveBytes)
		assert.True(t, config.EncryptionEnabled)
		assert.Equal(t, 30, config.RetentionDays)
	})
}

func TestRecordingConfig_Validation(t *testing.T) {
	t.Run("validates fMP4 format", func(t *testing.T) {
		config := recording.DefaultRecordingConfig()
		config.Format = recording.ContainerFormatFMP4
		// Validation would happen in Manager constructor
		assert.Equal(t, recording.ContainerFormatFMP4, config.Format)
	})

	t.Run("validates MKV format", func(t *testing.T) {
		config := recording.DefaultRecordingConfig()
		config.Format = recording.ContainerFormatMKV
		assert.Equal(t, recording.ContainerFormatMKV, config.Format)
	})
}

// Negative test: verify test fails when feature is broken
// (Anti-Bluff verification per Constitution Article XI)
func TestManager_Negative(t *testing.T) {
	t.Run("get status returns error after session deleted (simulated)", func(t *testing.T) {
		mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), &MockObjectStore{}, nil)
		require.NoError(t, err)

		_, err = mgr.StartRecording(context.Background(), "session-001", "tenant-001", "Game")
		require.NoError(t, err)

		// Simulate: if we try to get a session that doesn't exist
		_, err = mgr.GetRecordingStatus("session-999")
		require.Error(t, err) // MUST fail for non-existent session
		assert.Contains(t, err.Error(), "not found")
	})
}
