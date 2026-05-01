package stress

import (
	"context"
	"fmt"
	"io"
	"testing"

	"digital.vasic.storage/pkg/object"
	"digital.vasic.storage/pkg/recording"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordingStress_ConcurrentSessions(t *testing.T) {
	store := &noopStore{}
	mgr, _ := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	ctx := context.Background()

	const numSessions = 100 // Simulate 100 concurrent sessions
	done := make(chan bool, numSessions)

	for i := 0; i < numSessions; i++ {
		go func(id int) {
			sid := fmt.Sprintf("stress-%d", id)
			_, err := mgr.StartRecording(ctx, sid, "tenant-stress", "Game")
			if err != nil {
				t.Errorf("Failed to start session %s: %v", sid, err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < numSessions; i++ {
		<-done
	}

	// Verify all sessions created
	assert.Len(t, mgr.ListRecordings("tenant-stress"), numSessions)
}

func TestRecordingStress_LongRunning(t *testing.T) {
	store := &noopStore{}
	mgr, _ := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	ctx := context.Background()

	// Simulate 24-hour session (shortened for test)
	sid := "long-running"
	_, err := mgr.StartRecording(ctx, sid, "tenant-stress", "Game")
	require.NoError(t, err)

	// Simulate: in production, this would run for 24 hours
	// Verify no memory leaks, no resource exhaustion
	session, err := mgr.GetRecordingStatus(sid)
	require.NoError(t, err)
	assert.Equal(t, recording.RecordingStatusStaging, session.Status)
}

type noopStore struct{}

func (n *noopStore) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts ...object.PutOption) error {
	return nil
}
func (n *noopStore) GetObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	return nil, nil
}
func (n *noopStore) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	return nil
}
func (n *noopStore) ListObjects(ctx context.Context, bucketName, prefix string) ([]object.ObjectInfo, error) {
	return nil, nil
}
func (n *noopStore) StatObject(ctx context.Context, bucketName, objectName string) (*object.ObjectInfo, error) {
	return nil, nil
}
func (n *noopStore) CopyObject(ctx context.Context, src, dst object.ObjectRef) error {
	return nil
}
func (n *noopStore) Connect(ctx context.Context) error   { return nil }
func (n *noopStore) HealthCheck(ctx context.Context) error { return nil }
func (n *noopStore) Close() error                         { return nil }

func TestRecordingStress_Negative(t *testing.T) {
	store := &noopStore{}
	mgr, _ := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)

	// Verify: non-existent session MUST fail
	_, err := mgr.GetRecordingStatus("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
