// Copyright 2026 Vasic Digital. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package netstorage provides types and interfaces mirroring Storage-KMP
// for cross-platform network storage service abstractions.
package netstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

// StorageType identifies a storage protocol.
type StorageType string

const (
	StorageTypeWebDAV       StorageType = "WEBDAV"
	StorageTypeFTP          StorageType = "FTP"
	StorageTypeSFTP         StorageType = "SFTP"
	StorageTypeDropbox      StorageType = "DROPBOX"
	StorageTypeGoogleDrive  StorageType = "GOOGLE_DRIVE"
	StorageTypeOneDrive     StorageType = "ONEDRIVE"
	StorageTypeGit          StorageType = "GIT"
	StorageTypeS3           StorageType = "S3"
)

// AllStorageTypes returns all defined storage types.
func AllStorageTypes() []StorageType {
	return []StorageType{
		StorageTypeWebDAV, StorageTypeFTP, StorageTypeSFTP,
		StorageTypeDropbox, StorageTypeGoogleDrive, StorageTypeOneDrive,
		StorageTypeGit, StorageTypeS3,
	}
}

// DisplayName returns the human-readable name for the storage type.
func (s StorageType) DisplayName() string {
	switch s {
	case StorageTypeWebDAV:
		return "WebDAV"
	case StorageTypeFTP:
		return "FTP"
	case StorageTypeSFTP:
		return "SFTP"
	case StorageTypeDropbox:
		return "Dropbox"
	case StorageTypeGoogleDrive:
		return "Google Drive"
	case StorageTypeOneDrive:
		return "OneDrive"
	case StorageTypeGit:
		return "Git"
	case StorageTypeS3:
		return "S3"
	default:
		return string(s)
	}
}

// DefaultPort returns the default port for the storage type.
func (s StorageType) DefaultPort() int {
	switch s {
	case StorageTypeFTP:
		return 21
	case StorageTypeSFTP:
		return 22
	default:
		return 443
	}
}

// OperationStatus represents the status of a storage operation.
type OperationStatus string

const (
	OperationStatusPending    OperationStatus = "PENDING"
	OperationStatusInProgress OperationStatus = "IN_PROGRESS"
	OperationStatusCompleted  OperationStatus = "COMPLETED"
	OperationStatusFailed     OperationStatus = "FAILED"
	OperationStatusCancelled  OperationStatus = "CANCELLED"
	OperationStatusPaused     OperationStatus = "PAUSED"
)

// AllOperationStatuses returns all defined operation statuses.
func AllOperationStatuses() []OperationStatus {
	return []OperationStatus{
		OperationStatusPending, OperationStatusInProgress,
		OperationStatusCompleted, OperationStatusFailed,
		OperationStatusCancelled, OperationStatusPaused,
	}
}

// OperationType represents the type of storage operation.
type OperationType string

const (
	OperationTypeUpload       OperationType = "UPLOAD"
	OperationTypeDownload     OperationType = "DOWNLOAD"
	OperationTypeDelete       OperationType = "DELETE"
	OperationTypeMove         OperationType = "MOVE"
	OperationTypeCopy         OperationType = "COPY"
	OperationTypeCreateFolder OperationType = "CREATE_FOLDER"
	OperationTypeList         OperationType = "LIST"
	OperationTypeSync         OperationType = "SYNC"
)

// AllOperationTypes returns all defined operation types.
func AllOperationTypes() []OperationType {
	return []OperationType{
		OperationTypeUpload, OperationTypeDownload,
		OperationTypeDelete, OperationTypeMove,
		OperationTypeCopy, OperationTypeCreateFolder,
		OperationTypeList, OperationTypeSync,
	}
}

// ImplementationTier indicates the maturity of a protocol implementation.
type ImplementationTier string

const (
	TierProduction ImplementationTier = "PRODUCTION"
	TierBeta       ImplementationTier = "BETA"
	TierAlpha      ImplementationTier = "ALPHA"
	TierPlanned    ImplementationTier = "PLANNED"
)

// StorageDocument represents a file or directory in a storage service.
type StorageDocument struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	SizeBytes         int64  `json:"sizeBytes"`
	IsDirectory       bool   `json:"isDirectory"`
	MimeType          string `json:"mimeType,omitempty"`
	LastModifiedMillis *int64 `json:"lastModifiedMillis,omitempty"`
}

// text file extensions for type detection
var textExtensions = map[string]bool{
	"txt": true, "md": true, "markdown": true, "rst": true, "org": true,
	"tex": true, "csv": true, "tsv": true, "json": true, "xml": true,
	"yaml": true, "yml": true, "toml": true, "ini": true, "cfg": true,
	"conf": true, "properties": true, "log": true, "kt": true, "kts": true,
	"java": true, "py": true, "js": true, "ts": true, "html": true,
	"css": true, "sh": true, "bash": true, "zsh": true, "go": true,
	"rs": true, "c": true, "cpp": true, "h": true, "hpp": true,
	"swift": true, "rb": true, "php": true, "sql": true,
}

var imageExtensions = map[string]bool{
	"png": true, "jpg": true, "jpeg": true, "gif": true, "bmp": true,
	"svg": true, "webp": true, "ico": true, "tiff": true, "tif": true,
}

// Extension returns the file extension (without dot), or empty string.
func (d StorageDocument) Extension() string {
	ext := path.Ext(d.Name)
	if ext == "" {
		return ""
	}
	return strings.ToLower(ext[1:])
}

// FormattedSize returns human-readable file size.
func (d StorageDocument) FormattedSize() string {
	return formatBytes(d.SizeBytes)
}

// IsTextFile returns true if the file has a text extension.
func (d StorageDocument) IsTextFile() bool {
	return textExtensions[d.Extension()]
}

// IsImageFile returns true if the file has an image extension.
func (d StorageDocument) IsImageFile() bool {
	return imageExtensions[d.Extension()]
}

// IsInPath returns true if the document is under the given path.
func (d StorageDocument) IsInPath(dirPath string) bool {
	normalized := dirPath
	if !strings.HasSuffix(normalized, "/") {
		normalized += "/"
	}
	return strings.HasPrefix(d.Path, normalized)
}

// IsDirectChildOf returns true if the document is a direct child of the given directory.
func (d StorageDocument) IsDirectChildOf(dirPath string) bool {
	normalized := dirPath
	if !strings.HasSuffix(normalized, "/") {
		normalized += "/"
	}
	if !strings.HasPrefix(d.Path, normalized) {
		return false
	}
	remainder := d.Path[len(normalized):]
	return !strings.Contains(remainder, "/")
}

// StorageOperation represents a storage operation with progress tracking.
type StorageOperation struct {
	ID               string          `json:"id"`
	Type             OperationType   `json:"type"`
	Status           OperationStatus `json:"status"`
	RemotePath       string          `json:"remotePath"`
	LocalPath        string          `json:"localPath,omitempty"`
	TotalBytes       int64           `json:"totalBytes"`
	TransferredBytes int64           `json:"transferredBytes"`
	Error            string          `json:"error,omitempty"`
}

// ProgressPercent returns the progress percentage (0-100).
func (o StorageOperation) ProgressPercent() float64 {
	if o.TotalBytes <= 0 {
		return 0
	}
	return float64(o.TransferredBytes) / float64(o.TotalBytes) * 100
}

// IsComplete returns true if the operation has completed.
func (o StorageOperation) IsComplete() bool {
	return o.Status == OperationStatusCompleted
}

// IsFailed returns true if the operation has failed.
func (o StorageOperation) IsFailed() bool {
	return o.Status == OperationStatusFailed
}

// StorageInfo provides general storage service information.
type StorageInfo struct {
	DisplayName string      `json:"displayName"`
	StorageType StorageType `json:"storageType"`
	Host        string      `json:"host"`
	IsConnected bool        `json:"isConnected"`
}

// StorageQuota represents storage capacity information.
type StorageQuota struct {
	TotalBytes int64 `json:"totalBytes"`
	UsedBytes  int64 `json:"usedBytes"`
}

// AvailableBytes returns remaining capacity.
func (q StorageQuota) AvailableBytes() int64 {
	return q.TotalBytes - q.UsedBytes
}

// UsagePercent returns usage as a percentage (0-100).
func (q StorageQuota) UsagePercent() float64 {
	if q.TotalBytes <= 0 {
		return 0
	}
	return float64(q.UsedBytes) / float64(q.TotalBytes) * 100
}

// FormattedTotal returns human-readable total capacity.
func (q StorageQuota) FormattedTotal() string {
	return formatBytes(q.TotalBytes)
}

// FormattedUsed returns human-readable used space.
func (q StorageQuota) FormattedUsed() string {
	return formatBytes(q.UsedBytes)
}

// FormattedAvailable returns human-readable available space.
func (q StorageQuota) FormattedAvailable() string {
	return formatBytes(q.AvailableBytes())
}

// ProtocolInfo describes a storage protocol implementation.
type ProtocolInfo struct {
	StorageType StorageType        `json:"storageType"`
	DisplayName string             `json:"displayName"`
	Description string             `json:"description"`
	Tier        ImplementationTier `json:"tier"`
}

// NetworkStorageService defines the interface for network storage operations.
type NetworkStorageService interface {
	IsOnline() bool
	RootPath() string

	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	TestConnection(ctx context.Context) (bool, error)

	ListFiles(ctx context.Context, path string) ([]StorageDocument, error)
	DownloadFile(ctx context.Context, remotePath, localPath string) error
	UploadFile(ctx context.Context, localPath, remotePath string) error
	DeleteFile(ctx context.Context, remotePath string) error
	CreateFolder(ctx context.Context, remotePath string) (*StorageDocument, error)
	MoveFile(ctx context.Context, fromPath, toPath string) (*StorageDocument, error)
	CopyFile(ctx context.Context, fromPath, toPath string) (*StorageDocument, error)

	GetFileInfo(ctx context.Context, remotePath string) (*StorageDocument, error)
	GetQuota(ctx context.Context) (*StorageQuota, error)
	Search(ctx context.Context, query, searchPath string) ([]StorageDocument, error)

	NormalizePath(p string) string
	JoinPath(base, child string) string
	ParentPath(p string) string
}

// PlatformFileIO defines platform-specific file operations.
type PlatformFileIO interface {
	ReadFileBytes(ctx context.Context, path string) ([]byte, error)
	WriteFileBytes(ctx context.Context, path string, data []byte) error
	FileExists(ctx context.Context, path string) (bool, error)
	FileSize(ctx context.Context, path string) (int64, error)
	EnsureParentDirectories(ctx context.Context, path string) error
}

// MarshalJSON helpers for StorageDocument.
func (d StorageDocument) MarshalJSON() ([]byte, error) {
	type Alias StorageDocument
	return json.Marshal(Alias(d))
}

// MarshalJSON helpers for StorageOperation.
func (o StorageOperation) MarshalJSON() ([]byte, error) {
	type Alias StorageOperation
	return json.Marshal(Alias(o))
}

// formatBytes formats bytes into human-readable format.
func formatBytes(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
