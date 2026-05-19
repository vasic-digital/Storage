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

// ---------------------------------------------------------------------
// Round-216 (CONST-046 Phase 4 round 95) — 5 new provider-side sentinels
// covering the GCP + Azure validators / health checks migrated this
// round. Pattern mirrors the round-118 AWS sentinels above: real call
// site, captured-key assertion, NoopTranslator legacy-text check, no
// mock beyond the seam-recording fakeTranslator (CONST-050(A): mocks
// allowed only in unit tests).
// ---------------------------------------------------------------------

func TestNewGCPProvider_MissingProjectID_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{prefix: "X:"}
	provider.SetTranslator(ft)
	defer provider.SetTranslator(i18n.NoopTranslator{})

	_, err := provider.NewGCPProvider("", "us-central1", nil)
	if err == nil {
		t.Fatalf("expected error from missing project_id; got nil")
	}
	if ft.lastKey != "storage_provider_gcp_project_id_required" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
	if !strings.Contains(err.Error(), "storage_provider_gcp_project_id_required") {
		t.Fatalf("error %q does not carry seam payload", err.Error())
	}
}

func TestGCPProvider_HealthCheck_NoProjectID_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	provider.SetTranslator(ft)
	defer provider.SetTranslator(i18n.NoopTranslator{})

	p := &provider.GCPProvider{}
	err := p.HealthCheck(context.Background())
	if err == nil {
		t.Fatalf("expected error from empty GCPProvider HealthCheck; got nil")
	}
	if ft.lastKey != "storage_provider_gcp_project_id_not_configured" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
}

func TestNewAzureProvider_MissingSubscription_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	provider.SetTranslator(ft)
	defer provider.SetTranslator(i18n.NoopTranslator{})

	_, err := provider.NewAzureProvider("", "tenant", nil)
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if ft.lastKey != "storage_provider_azure_subscription_id_required" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
}

func TestNewAzureProvider_MissingTenant_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	provider.SetTranslator(ft)
	defer provider.SetTranslator(i18n.NoopTranslator{})

	_, err := provider.NewAzureProvider("sub", "", nil)
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if ft.lastKey != "storage_provider_azure_tenant_id_required" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
}

func TestAzureProvider_HealthCheck_NoCreds_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	provider.SetTranslator(ft)
	defer provider.SetTranslator(i18n.NoopTranslator{})

	p := &provider.AzureProvider{}
	err := p.HealthCheck(context.Background())
	if err == nil {
		t.Fatalf("expected error from empty AzureProvider HealthCheck; got nil")
	}
	if ft.lastKey != "storage_provider_azure_credentials_not_configured" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
}

// TestRound216_NoopDefault_PreservesLegacyText asserts every round-216
// production caller without a wired translator still sees the
// key-shaped error payload — same posture as the round-118 AWS
// sentinel. Paired-mutation: flipping `return key` to `return ""` in
// NoopTranslator.T must cause every assertion below to FAIL loud.
func TestRound216_NoopDefault_PreservesLegacyText(t *testing.T) {
	provider.SetTranslator(i18n.NoopTranslator{})

	tests := []struct {
		name        string
		call        func() error
		wantKey     string
	}{
		{
			name:    "gcp_missing_project_id",
			call:    func() error { _, e := provider.NewGCPProvider("", "us-central1", nil); return e },
			wantKey: "storage_provider_gcp_project_id_required",
		},
		{
			name:    "gcp_healthcheck_no_project",
			call:    func() error { return (&provider.GCPProvider{}).HealthCheck(context.Background()) },
			wantKey: "storage_provider_gcp_project_id_not_configured",
		},
		{
			name:    "azure_missing_subscription",
			call:    func() error { _, e := provider.NewAzureProvider("", "t", nil); return e },
			wantKey: "storage_provider_azure_subscription_id_required",
		},
		{
			name:    "azure_missing_tenant",
			call:    func() error { _, e := provider.NewAzureProvider("s", "", nil); return e },
			wantKey: "storage_provider_azure_tenant_id_required",
		},
		{
			name:    "azure_healthcheck_no_creds",
			call:    func() error { return (&provider.AzureProvider{}).HealthCheck(context.Background()) },
			wantKey: "storage_provider_azure_credentials_not_configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatalf("expected error; got nil")
			}
			if err.Error() != tc.wantKey {
				t.Fatalf("Noop default error = %q; want exact key %q",
					err.Error(), tc.wantKey)
			}
		})
	}
}
