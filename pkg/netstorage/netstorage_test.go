// Copyright 2026 Vasic Digital. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package netstorage

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageType_AllTypes(t *testing.T) {
	types := AllStorageTypes()
	assert.Len(t, types, 8)
}

func TestStorageType_DisplayName(t *testing.T) {
	tests := []struct {
		st   StorageType
		want string
	}{
		{StorageTypeWebDAV, "WebDAV"},
		{StorageTypeFTP, "FTP"},
		{StorageTypeSFTP, "SFTP"},
		{StorageTypeDropbox, "Dropbox"},
		{StorageTypeGoogleDrive, "Google Drive"},
		{StorageTypeOneDrive, "OneDrive"},
		{StorageTypeGit, "Git"},
		{StorageTypeS3, "S3"},
	}
	for _, tt := range tests {
		t.Run(string(tt.st), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.st.DisplayName())
		})
	}
}

func TestStorageType_DefaultPort(t *testing.T) {
	assert.Equal(t, 21, StorageTypeFTP.DefaultPort())
	assert.Equal(t, 22, StorageTypeSFTP.DefaultPort())
	assert.Equal(t, 443, StorageTypeWebDAV.DefaultPort())
	assert.Equal(t, 443, StorageTypeDropbox.DefaultPort())
	assert.Equal(t, 443, StorageTypeS3.DefaultPort())
}

func TestOperationStatus_AllStatuses(t *testing.T) {
	statuses := AllOperationStatuses()
	assert.Len(t, statuses, 6)
}

func TestOperationType_AllTypes(t *testing.T) {
	types := AllOperationTypes()
	assert.Len(t, types, 8)
}

func TestStorageDocument_Extension(t *testing.T) {
	doc := StorageDocument{Name: "notes.md"}
	assert.Equal(t, "md", doc.Extension())

	doc2 := StorageDocument{Name: "README"}
	assert.Equal(t, "", doc2.Extension())

	doc3 := StorageDocument{Name: "archive.TAR.GZ"}
	assert.Equal(t, "gz", doc3.Extension())
}

func TestStorageDocument_FormattedSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
		{-1, "0 B"},
	}
	for _, tt := range tests {
		doc := StorageDocument{SizeBytes: tt.bytes}
		assert.Equal(t, tt.want, doc.FormattedSize(), "bytes=%d", tt.bytes)
	}
}

func TestStorageDocument_IsTextFile(t *testing.T) {
	assert.True(t, StorageDocument{Name: "file.txt"}.IsTextFile())
	assert.True(t, StorageDocument{Name: "file.md"}.IsTextFile())
	assert.True(t, StorageDocument{Name: "file.go"}.IsTextFile())
	assert.True(t, StorageDocument{Name: "file.kt"}.IsTextFile())
	assert.False(t, StorageDocument{Name: "file.png"}.IsTextFile())
	assert.False(t, StorageDocument{Name: "file.exe"}.IsTextFile())
}

func TestStorageDocument_IsImageFile(t *testing.T) {
	assert.True(t, StorageDocument{Name: "photo.png"}.IsImageFile())
	assert.True(t, StorageDocument{Name: "photo.jpg"}.IsImageFile())
	assert.True(t, StorageDocument{Name: "icon.svg"}.IsImageFile())
	assert.False(t, StorageDocument{Name: "file.txt"}.IsImageFile())
}

func TestStorageDocument_IsInPath(t *testing.T) {
	doc := StorageDocument{Path: "/docs/notes/file.md"}
	assert.True(t, doc.IsInPath("/docs"))
	assert.True(t, doc.IsInPath("/docs/"))
	assert.True(t, doc.IsInPath("/docs/notes"))
	assert.False(t, doc.IsInPath("/other"))
}

func TestStorageDocument_IsDirectChildOf(t *testing.T) {
	doc := StorageDocument{Path: "/docs/file.md"}
	assert.True(t, doc.IsDirectChildOf("/docs"))
	assert.True(t, doc.IsDirectChildOf("/docs/"))
	assert.False(t, doc.IsDirectChildOf("/"))

	nested := StorageDocument{Path: "/docs/sub/file.md"}
	assert.False(t, nested.IsDirectChildOf("/docs"))
}

func TestStorageOperation_ProgressPercent(t *testing.T) {
	op := StorageOperation{TotalBytes: 1000, TransferredBytes: 500}
	assert.InDelta(t, 50.0, op.ProgressPercent(), 0.01)

	empty := StorageOperation{TotalBytes: 0}
	assert.Equal(t, 0.0, empty.ProgressPercent())
}

func TestStorageOperation_IsComplete(t *testing.T) {
	assert.True(t, StorageOperation{Status: OperationStatusCompleted}.IsComplete())
	assert.False(t, StorageOperation{Status: OperationStatusInProgress}.IsComplete())
}

func TestStorageOperation_IsFailed(t *testing.T) {
	assert.True(t, StorageOperation{Status: OperationStatusFailed}.IsFailed())
	assert.False(t, StorageOperation{Status: OperationStatusCompleted}.IsFailed())
}

func TestStorageQuota_AvailableBytes(t *testing.T) {
	q := StorageQuota{TotalBytes: 1000, UsedBytes: 300}
	assert.Equal(t, int64(700), q.AvailableBytes())
}

func TestStorageQuota_UsagePercent(t *testing.T) {
	q := StorageQuota{TotalBytes: 1000, UsedBytes: 250}
	assert.InDelta(t, 25.0, q.UsagePercent(), 0.01)

	empty := StorageQuota{TotalBytes: 0}
	assert.Equal(t, 0.0, empty.UsagePercent())
}

func TestStorageQuota_FormattedFields(t *testing.T) {
	q := StorageQuota{TotalBytes: 1073741824, UsedBytes: 536870912} // 1GB, 512MB
	assert.Equal(t, "1.0 GB", q.FormattedTotal())
	assert.Equal(t, "512.0 MB", q.FormattedUsed())
	assert.Equal(t, "512.0 MB", q.FormattedAvailable())
}

func TestStorageDocument_JSON(t *testing.T) {
	doc := StorageDocument{
		ID:          "abc",
		Name:        "test.txt",
		Path:        "/docs/test.txt",
		SizeBytes:   1024,
		IsDirectory: false,
	}
	data, err := json.Marshal(doc)
	require.NoError(t, err)

	var decoded StorageDocument
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, decoded.ID)
	assert.Equal(t, doc.Name, decoded.Name)
	assert.Equal(t, doc.SizeBytes, decoded.SizeBytes)
}

func TestStorageOperation_JSON(t *testing.T) {
	op := StorageOperation{
		ID:               "op1",
		Type:             OperationTypeUpload,
		Status:           OperationStatusInProgress,
		RemotePath:       "/remote/file.txt",
		TotalBytes:       2048,
		TransferredBytes: 1024,
	}
	data, err := json.Marshal(op)
	require.NoError(t, err)

	var decoded StorageOperation
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, op.ID, decoded.ID)
	assert.Equal(t, OperationTypeUpload, decoded.Type)
	assert.Equal(t, int64(1024), decoded.TransferredBytes)
}

func TestProtocolInfo_Fields(t *testing.T) {
	info := ProtocolInfo{
		StorageType: StorageTypeWebDAV,
		DisplayName: "WebDAV",
		Description: "Web Distributed Authoring and Versioning",
		Tier:        TierProduction,
	}
	assert.Equal(t, StorageTypeWebDAV, info.StorageType)
	assert.Equal(t, TierProduction, info.Tier)
}

func TestStorageInfo_Fields(t *testing.T) {
	info := StorageInfo{
		DisplayName: "My Cloud",
		StorageType: StorageTypeDropbox,
		Host:        "api.dropbox.com",
		IsConnected: true,
	}
	assert.Equal(t, "Dropbox", info.StorageType.DisplayName())
	assert.True(t, info.IsConnected)
}
