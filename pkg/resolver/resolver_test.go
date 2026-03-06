package resolver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackend is a test double that implements Backend.
type mockBackend struct {
	name    string
	data    map[string][]byte
	readErr error
	writeErr error
	existsErr error
	deleteErr error
}

func newMockBackend(name string) *mockBackend {
	return &mockBackend{
		name: name,
		data: make(map[string][]byte),
	}
}

func (m *mockBackend) Name() string { return m.name }

func (m *mockBackend) Read(
	_ context.Context, path string,
) (io.ReadCloser, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	d, ok := m.data[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return io.NopCloser(bytes.NewReader(d)), nil
}

func (m *mockBackend) Write(
	_ context.Context, path string, data io.Reader,
) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	b, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	m.data[path] = b
	return nil
}

func (m *mockBackend) Exists(
	_ context.Context, path string,
) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	_, ok := m.data[path]
	return ok, nil
}

func (m *mockBackend) Delete(
	_ context.Context, path string,
) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.data[path]; !ok {
		return fmt.Errorf("not found: %s", path)
	}
	delete(m.data, path)
	return nil
}

func TestNew(t *testing.T) {
	r := New()
	require.NotNil(t, r)
	assert.NotNil(t, r.backends)
	assert.Empty(t, r.rules)
	assert.Empty(t, r.fallback)
}

func TestRegisterBackend(t *testing.T) {
	r := New()
	b := newMockBackend("local")
	r.RegisterBackend(b)

	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Len(t, r.backends, 1)
	assert.Equal(t, b, r.backends["local"])
}

func TestResolve_ByRule(t *testing.T) {
	r := New()
	local := newMockBackend("local")
	s3 := newMockBackend("s3")
	r.RegisterBackend(local)
	r.RegisterBackend(s3)

	r.AddRule("/images/", "s3")
	r.AddRule("/docs/", "local")

	got, err := r.Resolve("/images/photo.jpg")
	require.NoError(t, err)
	assert.Equal(t, "s3", got.Name())

	got, err = r.Resolve("/docs/readme.txt")
	require.NoError(t, err)
	assert.Equal(t, "local", got.Name())
}

func TestResolve_Fallback(t *testing.T) {
	r := New()
	local := newMockBackend("local")
	r.RegisterBackend(local)
	r.SetFallback("local")

	got, err := r.Resolve("/unknown/path.bin")
	require.NoError(t, err)
	assert.Equal(t, "local", got.Name())
}

func TestResolve_NoMatch(t *testing.T) {
	r := New()
	local := newMockBackend("local")
	r.RegisterBackend(local)
	r.AddRule("/images/", "local")

	_, err := r.Resolve("/videos/clip.mp4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no backend matched path")
}

func TestRead(t *testing.T) {
	r := New()
	b := newMockBackend("local")
	b.data["/assets/file.txt"] = []byte("hello world")
	r.RegisterBackend(b)
	r.SetFallback("local")

	ctx := context.Background()
	rc, err := r.Read(ctx, "/assets/file.txt")
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
}

func TestWrite(t *testing.T) {
	r := New()
	b := newMockBackend("local")
	r.RegisterBackend(b)
	r.SetFallback("local")

	ctx := context.Background()
	err := r.Write(ctx, "/assets/new.txt", strings.NewReader("new content"))
	require.NoError(t, err)
	assert.Equal(t, []byte("new content"), b.data["/assets/new.txt"])
}

func TestExists(t *testing.T) {
	r := New()
	b := newMockBackend("local")
	b.data["/assets/exists.txt"] = []byte("data")
	r.RegisterBackend(b)
	r.SetFallback("local")

	ctx := context.Background()

	exists, err := r.Exists(ctx, "/assets/exists.txt")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = r.Exists(ctx, "/assets/missing.txt")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestDelete(t *testing.T) {
	r := New()
	b := newMockBackend("local")
	b.data["/assets/delete-me.txt"] = []byte("data")
	r.RegisterBackend(b)
	r.SetFallback("local")

	ctx := context.Background()
	err := r.Delete(ctx, "/assets/delete-me.txt")
	require.NoError(t, err)

	_, ok := b.data["/assets/delete-me.txt"]
	assert.False(t, ok)
}

func TestAddRule_Order(t *testing.T) {
	r := New()
	primary := newMockBackend("primary")
	secondary := newMockBackend("secondary")
	r.RegisterBackend(primary)
	r.RegisterBackend(secondary)

	// Both rules match "/images/photo.jpg" — first match wins.
	r.AddRule("/images/", "primary")
	r.AddRule("/images/photo", "secondary")

	got, err := r.Resolve("/images/photo.jpg")
	require.NoError(t, err)
	assert.Equal(t, "primary", got.Name(),
		"first matching rule should win")
}
