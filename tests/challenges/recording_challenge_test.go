package recording_challenge

import (
	"context"
	"testing"
	"time"

	"digital.vasic.storage/pkg/recording"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Challenge 1: Verify recording session lifecycle (C30 §2)
// Runs through full lifecycle: staging -> sealed -> synced
// Per R-14: Challenges (T11) + HelixQA (T12) mandatory
// Anti-Bluff: MUST fail when feature is commented out

func TestChallenge_RecordingLifecycle(t *testing.T) {
	store := &challengeStore{objects: make(map[string][]byte)}
	mgr, err := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Step 1: Start recording (staging)
	t.Run("start recording", func(t *testing.T) {
		session, err := mgr.StartRecording(ctx, "challenge-001", "tenant-challenge", "Cyberpunk 2077")
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, recording.RecordingStatusStaging, session.Status)
		t.Log("✓ Recording started in staging state")
	})

	// Step 2: Verify session exists with correct metadata
	t.Run("verify session metadata", func(t *testing.T) {
		session, err := mgr.GetRecordingStatus("challenge-001")
		require.NoError(t, err)
		assert.Equal(t, "tenant-challenge", session.TenantID)
		assert.Equal(t, "Cyberpunk 2077", session.GameTitle)
		assert.False(t, session.StartTime.IsZero())
		t.Log("✓ Session metadata correct")
	})

	// Step 3: Seal recording (simulates fMP4/MKV finalization)
	t.Run("seal recording", func(t *testing.T) {
		err := mgr.SealRecording(ctx, "challenge-001")
		require.NoError(t, err)

		session, err := mgr.GetRecordingStatus("challenge-001")
		require.NoError(t, err)
		assert.Equal(t, recording.RecordingStatusSealed, session.Status)
		assert.False(t, session.EndTime.IsZero())
		t.Log("✓ Recording sealed successfully")
	})

	// Step 4: Sync to backend (simulates S3/CloudFront upload)
	t.Run("sync to backend", func(t *testing.T) {
		err := mgr.SyncRecording(ctx, "challenge-001", "recordings-bucket")
		require.NoError(t, err)

		session, err := mgr.GetRecordingStatus("challenge-001")
		require.NoError(t, err)
		assert.Equal(t, recording.RecordingStatusSynced, session.Status)
		assert.NotEmpty(t, session.RemoteKey)
		assert.Contains(t, session.RemoteKey, "tenant-challenge/recordings/challenge-001")
		t.Log("✓ Recording synced to backend: " + session.RemoteKey)
	})

	// Step 5: Verify tenant isolation
	t.Run("tenant isolation", func(t *testing.T) {
		// Create another tenant's session
		mgr.StartRecording(ctx, "challenge-002", "tenant-other", "Game")
		recordings := mgr.ListRecordings("tenant-challenge")
		assert.Len(t, recordings, 1) // Only sees own recordings
		t.Log("✓ Tenant isolation enforced")
	})

	// Anti-Bluff: Verify challenge fails when feature is broken
	t.Run("anti-bluff: non-existent session fails", func(t *testing.T) {
		_, err := mgr.GetRecordingStatus("non-existent-session")
		require.Error(t, err) // MUST fail
		assert.Contains(t, err.Error(), "not found")
		t.Log("✓ Anti-bluff: correctly fails for non-existent session")
	})
}

type challengeStore struct {
	objects map[string][]byte
}

func (c *challengeStore) PutObject(ctx context.Context, bucketName, objectName string, reader interface{}, size int64, opts ...interface{}) error {
	data, ok := reader.([]byte)
	if !ok {
		return fmt.Errorf("reader must be []byte")
	}
	key := bucketName + "/" + objectName
	c.objects[key] = data
	return nil
}
func (c *challengeStore) GetObject(ctx context.Context, bucketName, objectName string) (interface{}, error) {
	key := bucketName + "/" + objectName
	data, exists := c.objects[key]
	if !exists {
		return nil, fmt.Errorf("not found")
	}
	return data, nil
}
func (c *challengeStore) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	key := bucketName + "/" + objectName
	delete(c.objects, key)
	return nil
}
func (c *challengeStore) ListObjects(ctx context.Context, bucketName, prefix string) ([]interface{}, error) {
	return nil, nil
}
func (c *challengeStore) StatObject(ctx context.Context, bucketName, objectName string) (interface{}, error) {
	return nil, nil
}
func (c *challengeStore) CopyObject(ctx context.Context, src, dst interface{}) error {
	return nil
}
func (c *challengeStore) HealthCheck(ctx context.Context) error { return nil }
func (c *challengeStore) Close() error                         { return nil }
