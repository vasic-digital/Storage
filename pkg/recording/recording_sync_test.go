// Package recording — round-37 §11.4 / CONST-035 anti-bluff coverage
// for the real S3 upload wiring in SyncRecording. Covers:
//
//   - Real-upload success path (mock satisfying object.ObjectStore;
//     mocks permitted in unit tests per CONST-050(A)).
//   - Operational-failure path → wraps ErrS3UploadFailed.
//   - Nil-client guard → returns ErrS3UploadNotWired (round-21
//     sentinel preserved for the misconfigured-deployment case).
//   - Paired mutation: assert errors.Is for both sentinels so a
//     future regression that swaps wrap layers gets caught.
//   - Directory-walk shape: real temp dir with N files; all bytes
//     accounted for in session.SizeBytes.
//
// The companion env-gated real-S3 integration test lives in
// pkg/recording/recording_sync_realbackend_test.go.
package recording

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"digital.vasic.storage/pkg/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeObjectStore is a hand-rolled in-memory object store satisfying
// object.ObjectStore for unit-test purposes. Permitted under
// CONST-050(A) (mocks/fakes in *_test.go are fine; integration tests
// use real backends — see recording_sync_realbackend_test.go).
type fakeObjectStore struct {
	mu             sync.Mutex
	uploads        map[string][]byte
	metadata       map[string]map[string]string
	putErr         error // if non-nil, all PutObject calls fail with this
	putCalls       int64
	putBytesTotal  int64
	putCalledKeys  []string
}

func newFakeObjectStore() *fakeObjectStore {
	return &fakeObjectStore{
		uploads:  make(map[string][]byte),
		metadata: make(map[string]map[string]string),
	}
}

func (f *fakeObjectStore) PutObject(
	ctx context.Context, bucket, key string,
	reader io.Reader, size int64, opts ...object.PutOption,
) error {
	atomic.AddInt64(&f.putCalls, 1)
	if f.putErr != nil {
		return f.putErr
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	storeKey := bucket + "/" + key
	f.uploads[storeKey] = data
	f.putCalledKeys = append(f.putCalledKeys, storeKey)
	atomic.AddInt64(&f.putBytesTotal, int64(len(data)))
	resolved := object.ResolvePutOptions(opts...)
	if resolved.Metadata != nil {
		f.metadata[storeKey] = resolved.Metadata
	}
	return nil
}

func (f *fakeObjectStore) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.uploads[bucket+"/"+key]
	if !ok {
		return nil, fmt.Errorf("object not found: %s/%s", bucket, key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (f *fakeObjectStore) DeleteObject(ctx context.Context, bucket, key string) error { return nil }
func (f *fakeObjectStore) ListObjects(ctx context.Context, bucket, prefix string) ([]object.ObjectInfo, error) {
	return nil, nil
}
func (f *fakeObjectStore) StatObject(ctx context.Context, bucket, key string) (*object.ObjectInfo, error) {
	return nil, nil
}
func (f *fakeObjectStore) CopyObject(ctx context.Context, src, dst object.ObjectRef) error {
	return nil
}
func (f *fakeObjectStore) Connect(ctx context.Context) error     { return nil }
func (f *fakeObjectStore) HealthCheck(ctx context.Context) error { return nil }
func (f *fakeObjectStore) Close() error                          { return nil }

// helper: create a sealed session whose ContainerPath is a real
// (potentially empty) tempdir, returning the manager + sessionID.
func newSealedSession(t *testing.T, store object.ObjectStore, populate func(dir string)) (*Manager, string) {
	t.Helper()
	cfg := DefaultRecordingConfig()
	tempRoot := t.TempDir()
	cfg.LocalStagingPath = tempRoot
	mgr, err := NewManager(cfg, store, nil)
	require.NoError(t, err)

	sessionID := "unit-sync-" + t.Name()
	_, err = mgr.StartRecording(context.Background(), sessionID, "tenant-unit", "Game")
	require.NoError(t, err)

	// Materialise the session's container directory (StartRecording
	// only builds the path string).
	session := mgr.sessions[sessionID]
	require.NoError(t, os.MkdirAll(session.ContainerPath, 0o755))
	if populate != nil {
		populate(session.ContainerPath)
	}

	require.NoError(t, mgr.SealRecording(context.Background(), sessionID))
	return mgr, sessionID
}

// TestSyncRecording_NilClient_ReturnsNotWired pins the round-21
// sentinel for the "no client configured" failure mode. The exported
// errors.Is contract MUST hold so callers can branch on it.
func TestSyncRecording_NilClient_ReturnsNotWired(t *testing.T) {
	cfg := DefaultRecordingConfig()
	cfg.LocalStagingPath = t.TempDir()
	mgr, err := NewManager(cfg, newFakeObjectStore(), nil)
	require.NoError(t, err)

	// Force nil client post-construction (constructor enforces non-nil,
	// but a wrapping layer could pass through nil; defence-in-depth
	// nil-guard MUST still fire).
	mgr.s3Client = nil

	sessionID := "nil-client-session"
	_, err = mgr.StartRecording(context.Background(), sessionID, "tenant", "Game")
	require.NoError(t, err)
	require.NoError(t, mgr.SealRecording(context.Background(), sessionID))

	syncErr := mgr.SyncRecording(context.Background(), sessionID, "any-bucket")
	require.Error(t, syncErr)
	// Paired-mutation invariant: errors.Is must hold even if the impl
	// changes to wrap the sentinel.
	assert.True(t, errors.Is(syncErr, ErrS3UploadNotWired),
		"expected errors.Is(err, ErrS3UploadNotWired) — got %v", syncErr)

	sess, err := mgr.GetRecordingStatus(sessionID)
	require.NoError(t, err)
	assert.Equal(t, RecordingStatusFailed, sess.Status,
		"nil-client failure MUST mark session Failed")
}

// TestSyncRecording_RealUploadSuccess_DirShape covers the directory-
// walk shape: real tempdir with two real files; both uploaded; bytes
// accounted for; session.Status flips to Synced.
func TestSyncRecording_RealUploadSuccess_DirShape(t *testing.T) {
	store := newFakeObjectStore()
	mgr, sessionID := newSealedSession(t, store, func(dir string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "init.mp4"), []byte("init-segment-bytes"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "segment-001.m4s"), []byte("segment-001-payload"), 0o644))
	})

	err := mgr.SyncRecording(context.Background(), sessionID, "recordings-bucket")
	require.NoError(t, err)

	sess, err := mgr.GetRecordingStatus(sessionID)
	require.NoError(t, err)
	assert.Equal(t, RecordingStatusSynced, sess.Status)
	assert.NotEmpty(t, sess.RemoteKey, "RemoteKey MUST be set after successful upload")
	assert.Equal(t, int64(len("init-segment-bytes")+len("segment-001-payload")), sess.SizeBytes,
		"SizeBytes MUST equal the real total of uploaded payloads")

	// Paired-mutation invariant: the fake actually received both PUTs.
	assert.Equal(t, int64(2), atomic.LoadInt64(&store.putCalls),
		"fake store MUST have received exactly 2 PutObject calls (one per file)")
	assert.Equal(t, int64(len("init-segment-bytes")+len("segment-001-payload")),
		atomic.LoadInt64(&store.putBytesTotal),
		"fake store MUST have observed the real byte total")
}

// TestSyncRecording_RealUploadSuccess_EmptyDir covers the empty-
// directory anti-bluff shape: a zero-byte marker MUST still be
// uploaded so callers see honest evidence of the upload attempt.
func TestSyncRecording_RealUploadSuccess_EmptyDir(t *testing.T) {
	store := newFakeObjectStore()
	mgr, sessionID := newSealedSession(t, store, nil) // empty dir

	err := mgr.SyncRecording(context.Background(), sessionID, "recordings-bucket")
	require.NoError(t, err)

	sess, err := mgr.GetRecordingStatus(sessionID)
	require.NoError(t, err)
	assert.Equal(t, RecordingStatusSynced, sess.Status)
	assert.Equal(t, int64(0), sess.SizeBytes)
	assert.Equal(t, int64(1), atomic.LoadInt64(&store.putCalls),
		"empty-dir MUST still produce exactly 1 PutObject (zero-byte marker)")
}

// TestSyncRecording_UploadFailure_WrapsErrS3UploadFailed exercises the
// operational-failure path: the fake returns an error from PutObject;
// SyncRecording MUST wrap it with ErrS3UploadFailed and mark session
// Failed.
func TestSyncRecording_UploadFailure_WrapsErrS3UploadFailed(t *testing.T) {
	store := newFakeObjectStore()
	store.putErr = errors.New("simulated network failure")
	mgr, sessionID := newSealedSession(t, store, func(dir string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "recording.mp4"), []byte("payload"), 0o644))
	})

	err := mgr.SyncRecording(context.Background(), sessionID, "recordings-bucket")
	require.Error(t, err)

	// Paired-mutation invariants:
	// (1) wraps ErrS3UploadFailed (the operational sentinel)
	assert.True(t, errors.Is(err, ErrS3UploadFailed),
		"expected errors.Is(err, ErrS3UploadFailed) — got %v", err)
	// (2) does NOT match ErrS3UploadNotWired (distinct meaning)
	assert.False(t, errors.Is(err, ErrS3UploadNotWired),
		"upload-failure MUST be distinguishable from nil-client; got %v", err)

	sess, err := mgr.GetRecordingStatus(sessionID)
	require.NoError(t, err)
	assert.Equal(t, RecordingStatusFailed, sess.Status,
		"operational upload failure MUST mark session Failed")
}

// TestSentinels_AreDistinctErrors guards against future refactor where
// someone aliases or merges the two sentinels — the round-21/round-37
// distinction is load-bearing for caller branching logic.
func TestSentinels_AreDistinctErrors(t *testing.T) {
	assert.NotSame(t, ErrS3UploadNotWired, ErrS3UploadFailed,
		"sentinels MUST be distinct error pointers")
	assert.False(t, errors.Is(ErrS3UploadNotWired, ErrS3UploadFailed),
		"errors.Is(NotWired, Failed) MUST be false")
	assert.False(t, errors.Is(ErrS3UploadFailed, ErrS3UploadNotWired),
		"errors.Is(Failed, NotWired) MUST be false")
}
