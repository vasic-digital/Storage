package recording_benchmark

import (
	"context"
	"testing"
	"time"

	"digital.vasic.storage/pkg/recording"
)

func BenchmarkManager_StartRecording(b *testing.B) {
	store := &MockObjectStoreNoop{}
	mgr, _ := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sid := string(rune('A' + i%26))
		mgr.StartRecording(ctx, "bench-"+sid, "tenant-bench", "Game")
	}
}

func BenchmarkManager_SealRecording(b *testing.B) {
	store := &MockObjectStoreNoop{}
	mgr, _ := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		sid := string(rune('A' + i%26))
		mgr.StartRecording(ctx, "bench-"+sid, "tenant", "Game")
		mgr.SealRecording(ctx, "bench-"+sid)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sid := string(rune('A' + i%26))
		mgr.SealRecording(ctx, "bench-"+sid)
	}
}

func BenchmarkManager_ListRecordings(b *testing.B) {
	store := &MockObjectStoreNoop{}
	mgr, _ := recording.NewManager(recording.DefaultRecordingConfig(), store, nil)
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		mgr.StartRecording(ctx, fmt.Sprintf("s-%d", i), "tenant", "Game")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.ListRecordings("tenant")
	}
}

type MockObjectStoreNoop struct{}

func (m *MockObjectStoreNoop) PutObject(ctx context.Context, bucketName, objectName string, reader interface{}, size int64, opts ...interface{}) error {
	return nil
}
func (m *MockObjectStoreNoop) GetObject(ctx context.Context, bucketName, objectName string) (interface{}, error) {
	return nil, nil
}
func (m *MockObjectStoreNoop) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	return nil
}
func (m *MockObjectStoreNoop) ListObjects(ctx context.Context, bucketName, prefix string) ([]interface{}, error) {
	return nil, nil
}
func (m *MockObjectStoreNoop) StatObject(ctx context.Context, bucketName, objectName string) (interface{}, error) {
	return nil, nil
}
func (m *MockObjectStoreNoop) CopyObject(ctx context.Context, src, dst interface{}) error {
	return nil
}
func (m *MockObjectStoreNoop) HealthCheck(ctx context.Context) error { return nil }
func (m *MockObjectStoreNoop) Close() error                         { return nil }
