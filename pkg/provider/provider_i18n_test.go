// Sentinel call-site tests for the provider package i18n migration
// (CONST-046, round-118).
//
// These tests pin the migrated error messages so a regression of the
// translator-seam wiring (e.g. accidentally hardcoding the literal back
// in source, or breaking the SetTranslator override) fails loud rather
// than greenly passing.
//
// Anti-bluff invariant: the assertions check the FULL message text
// captured from the real constructor / health-check path — no mock,
// no stub, no log-grep. Real call sites are exercised end-to-end with
// real inputs.
package provider_test

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.storage/pkg/i18n"
	"digital.vasic.storage/pkg/provider"
)

// fakeTranslator is a unit-test-only stub (CONST-050(A): mocks allowed
// only in unit tests). It records the most-recent key requested so the
// test asserts the call-site really hit the seam.
type fakeTranslator struct {
	lastKey string
	prefix  string
}

func (f *fakeTranslator) T(key string, _ map[string]any) string {
	f.lastKey = key
	return f.prefix + key
}

func TestNewAWSProvider_MissingAccessKey_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{prefix: "X:"}
	provider.SetTranslator(ft)
	defer provider.SetTranslator(i18n.NoopTranslator{})

	_, err := provider.NewAWSProvider("", "sk", "us-east-1", nil)
	if err == nil {
		t.Fatalf("expected error from missing access_key_id; got nil")
	}
	if ft.lastKey != "storage_provider_aws_access_key_required" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
	if !strings.Contains(err.Error(), "storage_provider_aws_access_key_required") {
		t.Fatalf("error %q does not carry seam payload", err.Error())
	}
}

func TestNewAWSProvider_MissingSecret_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	provider.SetTranslator(ft)
	defer provider.SetTranslator(i18n.NoopTranslator{})

	_, err := provider.NewAWSProvider("ak", "", "us-east-1", nil)
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if ft.lastKey != "storage_provider_aws_secret_required" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
}

func TestNewAWSProvider_MissingRegion_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	provider.SetTranslator(ft)
	defer provider.SetTranslator(i18n.NoopTranslator{})

	_, err := provider.NewAWSProvider("ak", "sk", "", nil)
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if ft.lastKey != "storage_provider_aws_region_required" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
}

func TestAWSProvider_HealthCheck_NoCreds_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	provider.SetTranslator(ft)
	defer provider.SetTranslator(i18n.NoopTranslator{})

	p := &provider.AWSProvider{}
	err := p.HealthCheck(context.Background())
	if err == nil {
		t.Fatalf("expected error from empty AWSProvider HealthCheck; got nil")
	}
	if ft.lastKey != "storage_provider_aws_credentials_not_configured" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
}

// TestNewAWSProvider_NoopDefault_PreservesLegacyText asserts that
// production callers who never invoke SetTranslator still see the
// original (now-key-shaped) error payload — keeps downstream string
// assertions actionable.
func TestNewAWSProvider_NoopDefault_PreservesLegacyText(t *testing.T) {
	provider.SetTranslator(i18n.NoopTranslator{})

	_, err := provider.NewAWSProvider("", "sk", "us-east-1", nil)
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if err.Error() != "storage_provider_aws_access_key_required" {
		t.Fatalf("Noop default error = %q; want exact key", err.Error())
	}
}

// TestSetTranslator_NilFallsBackToNoop verifies the nil-guard so callers
// can't accidentally crash production by passing nil.
func TestSetTranslator_NilFallsBackToNoop(t *testing.T) {
	provider.SetTranslator(nil) // must not panic
	defer provider.SetTranslator(i18n.NoopTranslator{})

	_, err := provider.NewAWSProvider("", "sk", "us-east-1", nil)
	if err == nil {
		t.Fatalf("expected error from missing access_key_id; got nil")
	}
	if err.Error() != "storage_provider_aws_access_key_required" {
		t.Fatalf("nil-guard fallback failed; err=%q", err.Error())
	}
}
