package resolver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Resolve: rule references missing backend ---

func TestResolve_RuleReferencesMissingBackend(t *testing.T) {
	r := New()
	// Add a rule that references a backend name that was never registered.
	r.AddRule("/images/", "nonexistent-backend")

	_, err := r.Resolve("/images/photo.jpg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backend \"nonexistent-backend\" referenced by rule")
	assert.Contains(t, err.Error(), "not found")
}

// --- Resolve: fallback references missing backend ---

func TestResolve_FallbackReferencesMissingBackend(t *testing.T) {
	r := New()
	r.SetFallback("missing-fallback")

	_, err := r.Resolve("/any/path.bin")
	require.Error(t, err)
	// round-216 / CONST-046: format key now lives in
	// pkg/i18n/bundles/active.en.yaml under
	// storage_resolver_fallback_backend_not_found ("fallback
	// backend %q not found"). NoopTranslator returns the key
	// verbatim (no %q verb), so fmt.Errorf carries the fallback
	// name as an EXTRA-string formatting note in the rendered
	// error. The migration sentinel just asserts both the key and
	// the backend name are present in the surfaced error string.
	assert.Contains(t, err.Error(), "storage_resolver_fallback_backend_not_found")
	assert.Contains(t, err.Error(), "missing-fallback")
}

// --- Read: resolve error propagation ---

func TestRead_ResolveError(t *testing.T) {
	r := New()
	// No backends, no rules, no fallback.
	_, err := r.Read(context.Background(), "/no/backend.txt")
	require.Error(t, err)
	// round-216 / CONST-046: legacy "resolve for read" English text now
	// keyed as storage_resolver_read_failure. NoopTranslator returns
	// the key verbatim and fmt.Errorf interprets it as the format
	// string; the %w verb retains error-wrapping semantics.
	assert.Contains(t, err.Error(), "storage_resolver_read_failure")
}

// --- Write: resolve error propagation ---

func TestWrite_ResolveError(t *testing.T) {
	r := New()
	err := r.Write(context.Background(), "/no/backend.txt", strings.NewReader("data"))
	require.Error(t, err)
	// round-216 / CONST-046: keyed as storage_resolver_write_failure.
	assert.Contains(t, err.Error(), "storage_resolver_write_failure")
}

// --- Exists: resolve error propagation ---

func TestExists_ResolveError(t *testing.T) {
	r := New()
	_, err := r.Exists(context.Background(), "/no/backend.txt")
	require.Error(t, err)
	// round-216 / CONST-046: keyed as storage_resolver_exists_failure.
	assert.Contains(t, err.Error(), "storage_resolver_exists_failure")
}

// --- Delete: resolve error propagation ---

func TestDelete_ResolveError(t *testing.T) {
	r := New()
	err := r.Delete(context.Background(), "/no/backend.txt")
	require.Error(t, err)
	// round-216 / CONST-046: keyed as storage_resolver_delete_failure.
	assert.Contains(t, err.Error(), "storage_resolver_delete_failure")
}

// --- Read: backend returns error ---

func TestRead_BackendError(t *testing.T) {
	r := New()
	b := newMockBackend("local")
	b.readErr = fmt.Errorf("disk I/O error")
	r.RegisterBackend(b)
	r.SetFallback("local")

	_, err := r.Read(context.Background(), "/assets/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk I/O error")
}

// --- Write: backend returns error ---

func TestWrite_BackendError(t *testing.T) {
	r := New()
	b := newMockBackend("local")
	b.writeErr = fmt.Errorf("permission denied")
	r.RegisterBackend(b)
	r.SetFallback("local")

	err := r.Write(context.Background(), "/assets/file.txt", strings.NewReader("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

// --- Exists: backend returns error ---

func TestExists_BackendError(t *testing.T) {
	r := New()
	b := newMockBackend("local")
	b.existsErr = fmt.Errorf("stat failed")
	r.RegisterBackend(b)
	r.SetFallback("local")

	_, err := r.Exists(context.Background(), "/assets/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stat failed")
}

// --- Delete: backend returns error ---

func TestDelete_BackendError(t *testing.T) {
	r := New()
	b := newMockBackend("local")
	b.deleteErr = fmt.Errorf("cannot remove")
	r.RegisterBackend(b)
	r.SetFallback("local")

	err := r.Delete(context.Background(), "/assets/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot remove")
}

// --- Resolve: rule with missing backend, then fallback with missing backend ---

func TestResolve_NoRuleMatch_NoFallback(t *testing.T) {
	r := New()
	r.RegisterBackend(newMockBackend("local"))
	// Add a rule that does NOT match the path.
	r.AddRule("/images/", "local")

	_, err := r.Resolve("/videos/clip.mp4")
	require.Error(t, err)
	// round-118 / CONST-046: NoopTranslator returns the key verbatim;
	// English text in pkg/i18n/bundles/active.en.yaml.
	assert.Contains(t, err.Error(), "storage_resolver_no_backend_matched")
}

// --- SetFallback: overwrite fallback ---

func TestSetFallback_Overwrite(t *testing.T) {
	r := New()
	r.RegisterBackend(newMockBackend("primary"))
	r.RegisterBackend(newMockBackend("secondary"))

	r.SetFallback("primary")
	b, err := r.Resolve("/any/path")
	require.NoError(t, err)
	assert.Equal(t, "primary", b.Name())

	r.SetFallback("secondary")
	b, err = r.Resolve("/any/path")
	require.NoError(t, err)
	assert.Equal(t, "secondary", b.Name())
}

// --- RegisterBackend: overwrite existing backend ---

func TestRegisterBackend_Overwrite(t *testing.T) {
	r := New()
	first := newMockBackend("store")
	second := newMockBackend("store")
	second.data["marker"] = []byte("second")

	r.RegisterBackend(first)
	r.RegisterBackend(second)

	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Len(t, r.backends, 1)
	_, hasMarker := r.backends["store"].(*mockBackend).data["marker"]
	assert.True(t, hasMarker, "second backend should have replaced first")
}
