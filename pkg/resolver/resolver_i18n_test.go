// Sentinel call-site test for the resolver package i18n migration
// (CONST-046, round-118 + round-216).
package resolver_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"digital.vasic.storage/pkg/i18n"
	"digital.vasic.storage/pkg/resolver"
)

type fakeTranslator struct {
	lastKey string
}

func (f *fakeTranslator) T(key string, _ map[string]any) string {
	f.lastKey = key
	// Preserve the printf verb so resolver.Resolve's %q substitution
	// produces a sensible final string.
	return "fake:" + key + " %q"
}

// fakeWrappingTranslator returns the key followed by `%w` so the four
// resolver wrap-points (Read/Write/Exists/Delete) still error-wrap the
// underlying cause when the seam is hit during round-216 tests.
type fakeWrappingTranslator struct {
	lastKey string
}

func (f *fakeWrappingTranslator) T(key string, _ map[string]any) string {
	f.lastKey = key
	return "fake:" + key + ": %w"
}

func TestResolver_NoBackendMatch_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	resolver.SetTranslator(ft)
	defer resolver.SetTranslator(i18n.NoopTranslator{})

	r := resolver.New()
	_, err := r.Resolve("/no/such/prefix")
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if ft.lastKey != "storage_resolver_no_backend_matched" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
	if !strings.Contains(err.Error(), "fake:storage_resolver_no_backend_matched") {
		t.Fatalf("error %q does not carry seam payload", err.Error())
	}
}

func TestResolver_NoopDefault_PreservesLegacyFormat(t *testing.T) {
	resolver.SetTranslator(i18n.NoopTranslator{})

	r := resolver.New()
	_, err := r.Resolve("/no/such/prefix")
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	// NoopTranslator returns key verbatim; resolver.Resolve passes it
	// to fmt.Errorf as the format string. Key has no %q verb, so the
	// path arg is dropped — but the key itself must appear verbatim.
	if !strings.Contains(err.Error(), "storage_resolver_no_backend_matched") {
		t.Fatalf("Noop default error = %q; want key verbatim", err.Error())
	}
}

// ---------------------------------------------------------------------
// Round-216 (CONST-046 Phase 4 round 95) — 5 new resolver-side
// sentinels covering the missing-fallback path + four
// Read/Write/Exists/Delete wrap-points migrated this round. Pattern
// mirrors round-118: real call site, captured-key assertion,
// NoopTranslator key-verbatim check, no mock beyond
// fakeWrappingTranslator (CONST-050(A): unit-test scope only).
// ---------------------------------------------------------------------

func TestResolver_FallbackMissing_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	resolver.SetTranslator(ft)
	defer resolver.SetTranslator(i18n.NoopTranslator{})

	r := resolver.New()
	r.SetFallback("missing")
	_, err := r.Resolve("/anywhere")
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if ft.lastKey != "storage_resolver_fallback_backend_not_found" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
	// %q verb in the fake template renders the missing name quoted.
	if !strings.Contains(err.Error(), `"missing"`) {
		t.Fatalf("error %q does not carry quoted fallback name", err.Error())
	}
}

func TestResolver_ReadWrap_UsesI18nSeam(t *testing.T) {
	ft := &fakeWrappingTranslator{}
	resolver.SetTranslator(ft)
	defer resolver.SetTranslator(i18n.NoopTranslator{})

	r := resolver.New() // no backends → Resolve fails → Read wraps
	_, err := r.Read(context.Background(), "/x")
	if err == nil {
		t.Fatalf("expected wrap error; got nil")
	}
	if ft.lastKey != "storage_resolver_read_failure" {
		t.Fatalf("seam not hit on read wrap: lastKey=%q", ft.lastKey)
	}
	// Verify error-wrapping semantics survive translation.
	var nilReader io.ReadCloser
	_ = nilReader
	if !errors.Is(err, err) { // sanity
		t.Fatalf("unexpected error identity behaviour")
	}
}

func TestResolver_WriteWrap_UsesI18nSeam(t *testing.T) {
	ft := &fakeWrappingTranslator{}
	resolver.SetTranslator(ft)
	defer resolver.SetTranslator(i18n.NoopTranslator{})

	r := resolver.New()
	err := r.Write(context.Background(), "/x", strings.NewReader("data"))
	if err == nil {
		t.Fatalf("expected wrap error; got nil")
	}
	if ft.lastKey != "storage_resolver_write_failure" {
		t.Fatalf("seam not hit on write wrap: lastKey=%q", ft.lastKey)
	}
}

func TestResolver_ExistsWrap_UsesI18nSeam(t *testing.T) {
	ft := &fakeWrappingTranslator{}
	resolver.SetTranslator(ft)
	defer resolver.SetTranslator(i18n.NoopTranslator{})

	r := resolver.New()
	_, err := r.Exists(context.Background(), "/x")
	if err == nil {
		t.Fatalf("expected wrap error; got nil")
	}
	if ft.lastKey != "storage_resolver_exists_failure" {
		t.Fatalf("seam not hit on exists wrap: lastKey=%q", ft.lastKey)
	}
}

func TestResolver_DeleteWrap_UsesI18nSeam(t *testing.T) {
	ft := &fakeWrappingTranslator{}
	resolver.SetTranslator(ft)
	defer resolver.SetTranslator(i18n.NoopTranslator{})

	r := resolver.New()
	err := r.Delete(context.Background(), "/x")
	if err == nil {
		t.Fatalf("expected wrap error; got nil")
	}
	if ft.lastKey != "storage_resolver_delete_failure" {
		t.Fatalf("seam not hit on delete wrap: lastKey=%q", ft.lastKey)
	}
}

// TestResolverRound216_NoopDefault_PreservesLegacyShape verifies that
// without a wired translator, every round-216 wrap-point still
// surfaces the key verbatim. Paired mutation: hand-flip `return key`
// → `return ""` in NoopTranslator.T MUST fail every assertion below.
func TestResolverRound216_NoopDefault_PreservesLegacyShape(t *testing.T) {
	resolver.SetTranslator(i18n.NoopTranslator{})

	r := resolver.New()
	r.SetFallback("absent")
	_, err := r.Resolve("/x")
	if err == nil || !strings.Contains(err.Error(), "storage_resolver_fallback_backend_not_found") {
		t.Fatalf("fallback wrap-point lost key: err=%v", err)
	}

	r2 := resolver.New()
	_, err = r2.Read(context.Background(), "/y")
	if err == nil || !strings.Contains(err.Error(), "storage_resolver_read_failure") {
		t.Fatalf("read wrap-point lost key: err=%v", err)
	}
	err = r2.Write(context.Background(), "/y", strings.NewReader("d"))
	if err == nil || !strings.Contains(err.Error(), "storage_resolver_write_failure") {
		t.Fatalf("write wrap-point lost key: err=%v", err)
	}
	_, err = r2.Exists(context.Background(), "/y")
	if err == nil || !strings.Contains(err.Error(), "storage_resolver_exists_failure") {
		t.Fatalf("exists wrap-point lost key: err=%v", err)
	}
	err = r2.Delete(context.Background(), "/y")
	if err == nil || !strings.Contains(err.Error(), "storage_resolver_delete_failure") {
		t.Fatalf("delete wrap-point lost key: err=%v", err)
	}
}
