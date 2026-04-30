package recording_smoke

import (
	"context"
	"testing"

	"digital.vasic.storage/pkg/recording"
	"github.com/stretchr/testify/assert"
)

func TestRecordingSmoke_QuickStart(t *testing.T) {
	store := &noopStore{}
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Quick smoke: start + seal should work in 30 seconds
	t.Run("start and seal recording", func(t *testing.T) {
		session, err := mgr.StartRecording(ctx, "smoke-001", "tenant-smoke", "Game")
		assert.NoError(t, err)
		assert.NotNil(t, session)

		err = mgr.SealRecording(ctx, "smoke-001")
		assert.NoError(t, err)

		session, err = mgr.GetRecordingStatus("smoke-001")
		assert.NoError(t, err)
		assert.Equal(t, recording.RecordingStatusSealed, session.Status)
	})
}

func TestRecordingSmoke_ListTenants(t *testing.T) {
	store := &noopStore{}
	mgr, _ := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	ctx := context.Background()

	mgr.StartRecording(ctx, "s1", "tenant-A", "Game")
	mgr.StartRecording(ctx, "s2", "tenant-A", "Game")
	mgr.StartRecording(ctx, "s3", "tenant-B", "Game")

	t.Run("list tenant A", func(t *testing.T) {
		recordings := mgr.ListRecordings("tenant-A")
		assert.Len(t, recordings, 2)
	})

	t.Run("list tenant B", func(t *testing.T) {
		recordings := mgr.ListRecordings("tenant-B")
		assert.Len(t, recordings, 1)
	})
}

type noopStore struct{}

func (n *noopStore) PutObject(ctx context.Context, bucketName, objectName string, reader interface{}, size int64, opts ...interface{}) error {
	return nil
}
func (n *noopStore) GetObject(ctx context.Context, bucketName, objectName string) (interface{}, error) {
	return nil, nil
}
func (n *noopStore) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	return nil
}
func (n *noopStore) ListObjects(ctx context.Context, bucketName, prefix string) ([]interface{}, error) {
	return nil, nil
}
func (n *noopStore) StatObject(ctx context.Context, bucketName, objectName string) (interface{}, error) {
	return nil, nil
}
func (n *noopStore) CopyObject(ctx context.Context, src, dst interface{}) error {
	return nil
}
func (n *noopStore) HealthCheck(ctx context.Context) error { return nil }
func (n *noopStore) Close() error                         { return nil }

func TestRecordingSmoke_Negative(t *testing.T) {
	store := &noopStore{}
	mgr, _ := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	ctx := context.Background()

	// Smoke: non-existent session MUST fail fast
	t.Run("non-existent session fails fast", func(t *testing.T) {
		_, err := mgr.GetRecordingStatus("fake-session")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
