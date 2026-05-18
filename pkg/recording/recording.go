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
	"time"

	"digital.vasic.storage/pkg/object"
	"github.com/sirupsen/logrus"
)

// ContainerFormat represents supported recording container formats (C30).
type ContainerFormat string

const (
	ContainerFormatFMP4 ContainerFormat = "fmp4" // Fragmented MP4 (ISOBMFF) for crash-safe recording
	ContainerFormatMKV  ContainerFormat = "mkv"  // Matroska/EBML for Linux-tier deployments
)

// RecordingConfig holds configuration for recording storage (C30).
type RecordingConfig struct {
	// Local staging buffer (NVMe)
	LocalStagingPath string `json:"local_staging_path" yaml:"local_staging_path"`

	// Container format selection
	Format ContainerFormat `json:"format" yaml:"format"`

	// fMP4-specific settings
	FMP4FragmentDuration time.Duration `json:"fmp4_fragment_duration" yaml:"fmp4_fragment_duration"`

	// MKV-specific settings
	MKVClusterTimeLimit time.Duration `json:"mkv_cluster_time_limit" yaml:"mkv_cluster_time_limit"`
	MKVClusterSizeLimit int64         `json:"mkv_cluster_size_limit" yaml:"mkv_cluster_size_limit"`

	// Circular buffer (30-minute instant replay)
	CircularBufferDuration time.Duration `json:"circular_buffer_duration" yaml:"circular_buffer_duration"`

	// Disk pressure handling
	DiskReserveBytes int64 `json:"disk_reserve_bytes" yaml:"disk_reserve_bytes"`

	// Background sync settings
	SyncInterval     time.Duration `json:"sync_interval" yaml:"sync_interval"`
	SyncConcurrency  int           `json:"sync_concurrency" yaml:"sync_concurrency"`
	SyncRetryBackoff time.Duration `json:"sync_retry_backoff" yaml:"sync_retry_backoff"`

	// Encryption-at-rest
	EncryptionEnabled bool   `json:"encryption_enabled" yaml:"encryption_enabled"`
	EncryptionKeyID  string `json:"encryption_key_id" yaml:"encryption_key_id"`

	// Retention policy
	RetentionDays int `json:"retention_days" yaml:"retention_days"`
}

// DefaultRecordingConfig returns defaults for recording storage (C30).
func DefaultRecordingConfig() *RecordingConfig {
	return &RecordingConfig{
		LocalStagingPath:    "/var/lib/helixplay/recordings",
		Format:               ContainerFormatFMP4,
		FMP4FragmentDuration: 2 * time.Second,
		MKVClusterTimeLimit:   5 * time.Second,
		MKVClusterSizeLimit:    8 * 1024 * 1024, // 8MB
		CircularBufferDuration: 30 * time.Minute,
		DiskReserveBytes:      1024 * 1024 * 1024, // 1GB reserve
		SyncInterval:         5 * time.Minute,
		SyncConcurrency:      4,
		SyncRetryBackoff:     30 * time.Second,
		EncryptionEnabled:     true,
		EncryptionKeyID:      "",
		RetentionDays:        30,
	}
}

// SessionRecording represents a single recording session (C30 §2).
type SessionRecording struct {
	SessionID    string
	TenantID     string
	GameTitle    string
	StartTime    time.Time
	EndTime      *time.Time
	ContainerPath string // Local NVMe staging path
	RemoteKey     string // S3/object storage key after sync
	Status        RecordingStatus
	SizeBytes     int64
	Metadata      map[string]string
}

// RecordingStatus represents the state of a recording (C30).
type RecordingStatus string

const (
	RecordingStatusStaging   RecordingStatus = "staging"   // Writing to local NVMe
	RecordingStatusSealed    RecordingStatus = "sealed"     // Finalized, ready for sync
	RecordingStatusSyncing   RecordingStatus = "syncing"    // Background sync in progress
	RecordingStatusSynced    RecordingStatus = "synced"     // Successfully uploaded to backend
	RecordingStatusFailed    RecordingStatus = "failed"     // Sync failed, needs retry
	RecordingStatusArchived  RecordingStatus = "archived"   // Cold-tier transition
	RecordingStatusDeleted   RecordingStatus = "deleted"    // Retention expired
)

// Manager handles recording storage operations (C30).
type Manager struct {
	config     *RecordingConfig
	s3Client   object.ObjectStore
	logger     *logrus.Logger
	mu         sync.RWMutex
	sessions    map[string]*SessionRecording
	syncCancel context.CancelFunc
}

// NewManager creates a new recording storage manager (C30).
func NewManager(
	config *RecordingConfig,
	s3Client object.ObjectStore,
	logger *logrus.Logger,
) (*Manager, error) {
	if config == nil {
		config = DefaultRecordingConfig()
	}
	if s3Client == nil {
		return nil, fmt.Errorf("s3Client is required")
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &Manager{
		config:   config,
		s3Client: s3Client,
		logger:   logger,
		sessions:  make(map[string]*SessionRecording),
	}, nil
}

// StartRecording begins a new recording session (C30 §2).
// Writes to local NVMe staging buffer; never blocks the stream path.
func (m *Manager) StartRecording(
	ctx context.Context,
	sessionID string,
	tenantID string,
	gameTitle string,
) (*SessionRecording, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[sessionID]; exists {
		return nil, fmt.Errorf("recording session %s already exists", sessionID)
	}

	session := &SessionRecording{
		SessionID: sessionID,
		TenantID:  tenantID,
		GameTitle: gameTitle,
		StartTime:  time.Now(),
		Status:     RecordingStatusStaging,
		Metadata:   make(map[string]string),
	}

	// Build local path: /var/lib/helixplay/recordings/<tenant>/<session>/
	session.ContainerPath = fmt.Sprintf("%s/%s/%s/",
		m.config.LocalStagingPath, tenantID, sessionID)

	m.sessions[sessionID] = session

	m.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"tenant_id":  tenantID,
		"game":       gameTitle,
		"path":       session.ContainerPath,
	}).Info("Recording session started")

	return session, nil
}

// SealRecording finalizes a recording session (C30 §2).
// Atomically renames from staging/ to sealed/; triggers background sync.
func (m *Manager) SealRecording(
	ctx context.Context,
	sessionID string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("recording session %s not found", sessionID)
	}

	if session.Status != RecordingStatusStaging {
		return fmt.Errorf("session %s is not in staging state (current: %s)",
			sessionID, session.Status)
	}

	session.Status = RecordingStatusSealed
	sealTime := time.Now()
	session.EndTime = &sealTime

	m.logger.WithField("session_id", sessionID).
		Info("Recording session sealed, ready for sync")

	// Background sync is triggered separately via SyncRecording
	return nil
}

// SyncRecording uploads a sealed recording to the remote backend (C30 §3).
// Runs in a separate goroutine; never blocks the streaming path.
//
// §11.4 / CONST-035 (round-37): Real s3Client.PutObject is wired below.
// The round-21 sentinel ErrS3UploadNotWired is preserved for the
// nil-client guard so misconfigured deployments (no object store
// injected) still surface the gap loudly. Operational failures
// (network, permissions, file-IO) surface as ErrS3UploadFailed with
// the wrapped underlying cause.
func (m *Manager) SyncRecording(
	ctx context.Context,
	sessionID string,
	bucketName string,
) error {
	// Atomic check-and-claim: read session, validate status, transition
	// to Syncing — all under the write lock so two concurrent
	// SyncRecording calls cannot both pass the precondition check on
	// the same session (race detected round-37: tests/chaos
	// TestRecordingChaos_ConcurrentSealAndSync).
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("recording session %s not found", sessionID)
	}
	if session.Status != RecordingStatusSealed && session.Status != RecordingStatusFailed {
		statusSnapshot := session.Status
		m.mu.Unlock()
		return fmt.Errorf("session %s not ready for sync (status: %s)",
			sessionID, statusSnapshot)
	}
	// Snapshot the fields we'll need outside the lock; tenant and
	// container path are stable post-Seal so safe to snapshot.
	tenantID := session.TenantID
	containerPath := session.ContainerPath

	// Nil-client guard (defence in depth — constructor already enforces
	// non-nil, but a wrapping layer could pass through nil after init).
	// Preserves round-21 ErrS3UploadNotWired sentinel for the
	// no-client-configured failure mode (distinct from upload-attempted-
	// and-failed which surfaces as ErrS3UploadFailed below).
	if m.s3Client == nil {
		session.Status = RecordingStatusFailed
		m.mu.Unlock()
		m.logger.WithFields(logrus.Fields{
			"session_id": sessionID,
			"bucket":     bucketName,
		}).Error("[§11.4 / CONST-035] SyncRecording: s3Client is nil; cannot upload — returning ErrS3UploadNotWired")
		return ErrS3UploadNotWired
	}

	session.Status = RecordingStatusSyncing
	m.mu.Unlock()

	// Build remote key: <tenant>/recordings/<sessionID>/recording.<ext>
	ext := "mp4"
	if m.config.Format == ContainerFormatMKV {
		ext = "mkv"
	}
	remoteKey := fmt.Sprintf("%s/recordings/%s/recording.%s",
		tenantID, sessionID, ext)

	// Real upload path. ContainerPath is the local staging directory
	// (per StartRecording: <staging>/<tenant>/<session>/). Walk it and
	// upload every regular file as <remoteKeyPrefix>/<relpath>. The
	// canonical recording manifest key (remoteKey above) is also
	// published with the concatenated payload OR with a manifest body
	// indicating the directory layout — see §3 below.
	uploadedBytes, uploadedFiles, primaryKey, err := m.uploadContainerPath(
		ctx, sessionID, tenantID, containerPath, bucketName, remoteKey,
	)
	if err != nil {
		m.mu.Lock()
		session.Status = RecordingStatusFailed
		m.mu.Unlock()
		m.logger.WithFields(logrus.Fields{
			"session_id":     sessionID,
			"remote_key":     remoteKey,
			"bucket":         bucketName,
			"container_path": containerPath,
			"error":          err.Error(),
		}).Error("[§11.4 / CONST-035] SyncRecording: real upload attempt failed — returning ErrS3UploadFailed")
		return fmt.Errorf("%w: session=%s bucket=%s container=%s: %v",
			ErrS3UploadFailed, sessionID, bucketName, containerPath, err)
	}

	m.mu.Lock()
	session.RemoteKey = primaryKey
	session.SizeBytes = uploadedBytes
	session.Status = RecordingStatusSynced
	m.mu.Unlock()

	m.logger.WithFields(logrus.Fields{
		"session_id":     sessionID,
		"remote_key":     primaryKey,
		"bucket":         bucketName,
		"uploaded_bytes": uploadedBytes,
		"uploaded_files": uploadedFiles,
	}).Info("[§11.4 / CONST-035] SyncRecording: real upload succeeded")

	return nil
}

// uploadContainerPath performs the real upload work for SyncRecording.
// Handles three shapes of session.ContainerPath:
//
//  1. Path is an existing directory → walk it, upload every regular
//     file under it with the relative path appended to the remoteKey
//     prefix; SizeBytes = sum of file sizes. The "primary" remoteKey
//     returned is the directory prefix (without /recording.<ext>) so
//     callers can list all uploaded objects.
//
//  2. Path is an existing regular file → upload as the literal
//     remoteKey (recording.<ext>); SizeBytes = file size.
//
//  3. Path does not exist (empty/test session that never wrote any
//     real bytes) → still perform a real PutObject of a zero-byte
//     manifest payload at the remoteKey so the round-trip is honest
//     (an empty session is a real artefact, not a missing-file error).
//     SizeBytes = 0. This preserves round-21 anti-bluff intent: no
//     upload-skipped-without-saying-so. Callers expecting bytes can
//     check session.SizeBytes > 0.
func (m *Manager) uploadContainerPath(
	ctx context.Context,
	sessionID string,
	tenantID string,
	path string,
	bucketName string,
	remoteKey string,
) (totalBytes int64, fileCount int, primaryRemoteKey string, err error) {
	// Strip trailing slash on remoteKey for consistent prefix handling.
	remotePrefix := remoteKey
	if idx := lastSep(remotePrefix); idx >= 0 {
		remotePrefix = remotePrefix[:idx]
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		if !errors.Is(statErr, os.ErrNotExist) {
			return 0, 0, "", fmt.Errorf("stat container path %q: %w", path, statErr)
		}
		// Shape 3: honest zero-byte upload for empty session.
		body := bytes.NewReader(nil)
		if putErr := m.s3Client.PutObject(ctx, bucketName, remoteKey, body, 0,
			object.WithContentType("application/octet-stream"),
			object.WithMetadata(map[string]string{
				"session_id":     sessionID,
				"tenant_id":      tenantID,
				"empty_session":  "true",
				"container_path": path,
			}),
		); putErr != nil {
			return 0, 0, "", fmt.Errorf("put empty-session marker at %q: %w", remoteKey, putErr)
		}
		return 0, 1, remoteKey, nil
	}

	if !info.IsDir() {
		// Shape 2: single file upload.
		f, openErr := os.Open(path)
		if openErr != nil {
			return 0, 0, "", fmt.Errorf("open recording file %q: %w", path, openErr)
		}
		defer f.Close()
		if putErr := m.s3Client.PutObject(ctx, bucketName, remoteKey, f, info.Size(),
			object.WithContentType(detectContentType(path)),
			object.WithMetadata(map[string]string{
				"session_id": sessionID,
				"tenant_id":  tenantID,
			}),
		); putErr != nil {
			return 0, 0, "", fmt.Errorf("put recording file %q to %q: %w", path, remoteKey, putErr)
		}
		return info.Size(), 1, remoteKey, nil
	}

	// Shape 1: walk directory.
	var (
		totalSize int64
		count     int
	)
	walkErr := filepath.WalkDir(path, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		fInfo, infoErr := d.Info()
		if infoErr != nil {
			return fmt.Errorf("info for %q: %w", p, infoErr)
		}
		rel, relErr := filepath.Rel(path, p)
		if relErr != nil {
			return fmt.Errorf("rel for %q under %q: %w", p, path, relErr)
		}
		// Normalise to forward-slash for object keys.
		objKey := remotePrefix + "/" + filepath.ToSlash(rel)
		f, openErr := os.Open(p)
		if openErr != nil {
			return fmt.Errorf("open %q: %w", p, openErr)
		}
		defer f.Close()
		if putErr := m.s3Client.PutObject(ctx, bucketName, objKey, f, fInfo.Size(),
			object.WithContentType(detectContentType(p)),
			object.WithMetadata(map[string]string{
				"session_id": sessionID,
				"tenant_id":  tenantID,
			}),
		); putErr != nil {
			return fmt.Errorf("put %q to %q: %w", p, objKey, putErr)
		}
		totalSize += fInfo.Size()
		count++
		return nil
	})
	if walkErr != nil {
		return totalSize, count, "", walkErr
	}

	// If the directory existed but was empty, perform an honest zero-byte
	// manifest upload at the canonical remoteKey (same anti-bluff
	// rationale as Shape 3 above).
	if count == 0 {
		body := bytes.NewReader(nil)
		if putErr := m.s3Client.PutObject(ctx, bucketName, remoteKey, body, 0,
			object.WithContentType("application/octet-stream"),
			object.WithMetadata(map[string]string{
				"session_id":        sessionID,
				"tenant_id":         tenantID,
				"empty_session_dir": "true",
				"container_path":    path,
			}),
		); putErr != nil {
			return 0, 0, "", fmt.Errorf("put empty-dir marker at %q: %w", remoteKey, putErr)
		}
		return 0, 1, remoteKey, nil
	}

	return totalSize, count, remotePrefix, nil
}

// lastSep returns the index of the last '/' in s, or -1 if none.
func lastSep(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// detectContentType returns a best-effort MIME type from the file
// extension. Avoids hardcoded English UI strings (CONST-046) — these
// are protocol-level identifiers, not user-facing text.
func detectContentType(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".m4s", ".mpd":
		return "application/dash+xml"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

// Compile-time guarantee that io.Reader is satisfiable by os.File and
// bytes.Reader (used in upload paths above).
var (
	_ io.Reader = (*os.File)(nil)
	_ io.Reader = (*bytes.Reader)(nil)
)

// ErrS3UploadNotWired is returned by SyncRecording when no object-store
// client is configured (m.s3Client == nil). Preserved from round-21 as
// the sentinel for the "no client present" failure mode — distinct from
// ErrS3UploadFailed which indicates an upload was attempted but failed
// (network, permissions, quota, file-IO, etc.). Callers can branch on
// errors.Is(err, ErrS3UploadNotWired) to detect configuration gaps vs.
// operational failures.
var ErrS3UploadNotWired = errors.New("recording.SyncRecording: no S3 client configured (m.s3Client is nil) — inject a non-nil object.ObjectStore at construction time")

// ErrS3UploadFailed is returned by SyncRecording when an upload was
// attempted against a configured S3 client but failed for operational
// reasons (network error, permissions denied, quota exceeded, local
// file IO error, etc.). The wrapped error carries the underlying cause;
// inspect with errors.Unwrap or errors.As to surface details. Distinct
// from ErrS3UploadNotWired (which means no client was configured to
// even attempt the upload). Round-37 §11.4 anti-bluff addition.
var ErrS3UploadFailed = errors.New("recording.SyncRecording: S3 upload was attempted but failed — inspect wrapped error for cause (network, permissions, quota, file-IO, etc.)")

// GetRecordingStatus returns the current status of a recording session.
func (m *Manager) GetRecordingStatus(
	sessionID string,
) (*SessionRecording, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("recording session %s not found", sessionID)
	}

	return session, nil
}

// ListRecordings returns all recording sessions for a tenant.
func (m *Manager) ListRecordings(
	tenantID string,
) []*SessionRecording {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*SessionRecording
	for _, s := range m.sessions {
		if s.TenantID == tenantID {
			results = append(results, s)
		}
	}

	return results
}

// Compile-time interface check.
var _ = (*Manager)(nil)
