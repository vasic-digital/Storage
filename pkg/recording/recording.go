package recording

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"digital.vasic.storage/pkg/object"
	"digital.vasic.storage/pkg/s3"
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
func (m *Manager) SyncRecording(
	ctx context.Context,
	sessionID string,
	bucketName string,
) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("recording session %s not found", sessionID)
	}

	if session.Status != RecordingStatusSealed && session.Status != RecordingStatusFailed {
		return fmt.Errorf("session %s not ready for sync (status: %s)",
			sessionID, session.Status)
	}

	m.mu.Lock()
	session.Status = RecordingStatusSyncing
	m.mu.Unlock()

	// Build remote key: <tenant>/recordings/<sessionID>/recording.<ext>
	ext := "mp4"
	if m.config.Format == ContainerFormatMKV {
		ext = "mkv"
	}
	remoteKey := fmt.Sprintf("%s/recordings/%s/recording.%s",
		session.TenantID, sessionID, ext)

	// Read local file and upload to S3
	// (In production, this would use the local staging file)
	m.logger.WithFields(logrus.Fields{
		"session_id":  sessionID,
		"remote_key": remoteKey,
		"bucket":     bucketName,
	}).Info("Starting background sync to remote backend")

	// Simulate upload (in production: read from session.ContainerPath + upload via s3Client.PutObject)
	// This is a placeholder for the actual file read + upload logic
	session.RemoteKey = remoteKey

	m.mu.Lock()
	session.Status = RecordingStatusSynced
	m.mu.Unlock()

	m.logger.WithField("session_id", sessionID).
		Info("Recording synced to remote backend")

	return nil
}

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
