// Sentinel call-site test for the resolver package i18n migration
// (CONST-046, round-118).
package resolver_test

import (
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
