// Tests for the Storage i18n seam (CONST-046, round-118).
//
// Covers (per §11.4 anti-bluff posture):
//   - NoopTranslator returns the key verbatim regardless of params.
//   - Translator interface is satisfied by NoopTranslator (compile-time
//     guard + runtime assertion).
//   - Mutation-resistance: a sentinel hand-edit of the noop's return
//     statement would flip every assertion below — the test fails loud
//     rather than greenly accepting an empty-string regression.
package i18n_test

import (
	"testing"

	"digital.vasic.storage/pkg/i18n"
)

func TestNoopTranslator_ReturnsKeyVerbatim(t *testing.T) {
	t.Parallel()
	noop := i18n.NoopTranslator{}

	cases := []string{
		"storage_provider_aws_access_key_required",
		"storage_provider_aws_secret_required",
		"storage_provider_aws_region_required",
		"storage_provider_aws_credentials_not_configured",
		"storage_resolver_no_backend_matched",
		"",                                // empty key — must still echo
		"some_unknown_key_with_unicode_é", // unicode safety
	}
	for _, key := range cases {
		got := noop.T(key, nil)
		if got != key {
			t.Fatalf("NoopTranslator.T(%q) = %q; want verbatim key", key, got)
		}
	}
}

func TestNoopTranslator_IgnoresParams(t *testing.T) {
	t.Parallel()
	noop := i18n.NoopTranslator{}
	const key = "storage_provider_aws_access_key_required"
	params := map[string]any{"path": "/var/run", "code": 42}
	if got := noop.T(key, params); got != key {
		t.Fatalf("NoopTranslator.T with params = %q; want key verbatim %q", got, key)
	}
}

func TestNoopTranslator_SatisfiesInterface(t *testing.T) {
	t.Parallel()
	var tr i18n.Translator = i18n.NoopTranslator{} // compile-time guard
	if got := tr.T("k", nil); got != "k" {
		t.Fatalf("via interface: got %q; want %q", got, "k")
	}
}
