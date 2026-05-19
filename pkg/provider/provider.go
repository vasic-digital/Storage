package provider

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.storage/pkg/i18n"
)

// translator is the package-level message-resolution seam (CONST-046,
// round-118). Defaults to NoopTranslator so production behaviour is
// unchanged for any caller that has not wired a project-side
// translator; consumers swap it via SetTranslator at startup.
//
// Per CONST-051(B) the seam is project-not-aware: this package never
// imports a HelixCode-specific catalogue; the consuming project is
// responsible for providing the real Translator implementation.
var translator i18n.Translator = i18n.NoopTranslator{}

// SetTranslator wires a project-side Translator. Callers MUST invoke it
// at startup before issuing user-facing error returns; calling it
// concurrently with provider construction is not supported.
func SetTranslator(t i18n.Translator) {
	if t == nil {
		t = i18n.NoopTranslator{}
	}
	translator = t
}

// CloudProvider defines the interface for cloud provider credential and
// health management.
type CloudProvider interface {
	// Name returns the provider name.
	Name() string

	// Credentials returns the provider credentials as a key-value map.
	Credentials() map[string]string

	// HealthCheck verifies connectivity to the cloud provider.
	HealthCheck(ctx context.Context) error
}

// ProviderConfig holds common configuration for cloud providers.
type ProviderConfig struct {
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// DefaultProviderConfig returns a ProviderConfig with sensible defaults.
func DefaultProviderConfig() *ProviderConfig {
	return &ProviderConfig{
		Timeout: 30 * time.Second,
	}
}

// AWSProvider manages AWS credentials and connectivity.
type AWSProvider struct {
	AccessKeyID     string `json:"access_key_id" yaml:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key" yaml:"secret_access_key"`
	Region          string `json:"region" yaml:"region"`
	SessionToken    string `json:"session_token,omitempty" yaml:"session_token"`
	config          *ProviderConfig
}

// NewAWSProvider creates a new AWS provider with the given credentials.
func NewAWSProvider(
	accessKeyID string,
	secretAccessKey string,
	region string,
	config *ProviderConfig,
) (*AWSProvider, error) {
	if accessKeyID == "" {
		return nil, fmt.Errorf("%s", translator.T("storage_provider_aws_access_key_required", nil))
	}
	if secretAccessKey == "" {
		return nil, fmt.Errorf("%s", translator.T("storage_provider_aws_secret_required", nil))
	}
	if region == "" {
		return nil, fmt.Errorf("%s", translator.T("storage_provider_aws_region_required", nil))
	}
	if config == nil {
		config = DefaultProviderConfig()
	}

	return &AWSProvider{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		Region:          region,
		config:          config,
	}, nil
}

// Name returns "aws".
func (p *AWSProvider) Name() string {
	return "aws"
}

// Credentials returns the AWS credentials.
func (p *AWSProvider) Credentials() map[string]string {
	creds := map[string]string{
		"access_key_id":     p.AccessKeyID,
		"secret_access_key": p.SecretAccessKey,
		"region":            p.Region,
	}
	if p.SessionToken != "" {
		creds["session_token"] = p.SessionToken
	}
	return creds
}

// HealthCheck verifies that AWS credentials are configured.
func (p *AWSProvider) HealthCheck(_ context.Context) error {
	if p.AccessKeyID == "" || p.SecretAccessKey == "" {
		return fmt.Errorf("%s", translator.T("storage_provider_aws_credentials_not_configured", nil))
	}
	return nil
}

// WithSessionToken sets a session token for temporary credentials.
func (p *AWSProvider) WithSessionToken(token string) *AWSProvider {
	p.SessionToken = token
	return p
}

// GCPProvider manages GCP credentials and connectivity.
type GCPProvider struct {
	ProjectID      string `json:"project_id" yaml:"project_id"`
	ServiceAccount string `json:"service_account,omitempty" yaml:"service_account"`
	Location       string `json:"location" yaml:"location"`
	config         *ProviderConfig
}

// NewGCPProvider creates a new GCP provider with the given credentials.
func NewGCPProvider(
	projectID string,
	location string,
	config *ProviderConfig,
) (*GCPProvider, error) {
	if projectID == "" {
		return nil, fmt.Errorf("%s", translator.T("storage_provider_gcp_project_id_required", nil))
	}
	if location == "" {
		location = "us-central1"
	}
	if config == nil {
		config = DefaultProviderConfig()
	}

	return &GCPProvider{
		ProjectID: projectID,
		Location:  location,
		config:    config,
	}, nil
}

// Name returns "gcp".
func (p *GCPProvider) Name() string {
	return "gcp"
}

// Credentials returns the GCP credentials.
func (p *GCPProvider) Credentials() map[string]string {
	creds := map[string]string{
		"project_id": p.ProjectID,
		"location":   p.Location,
	}
	if p.ServiceAccount != "" {
		creds["service_account"] = p.ServiceAccount
	}
	return creds
}

// HealthCheck verifies that GCP credentials are configured.
func (p *GCPProvider) HealthCheck(_ context.Context) error {
	if p.ProjectID == "" {
		return fmt.Errorf("%s", translator.T("storage_provider_gcp_project_id_not_configured", nil))
	}
	return nil
}

// WithServiceAccount sets the service account email.
func (p *GCPProvider) WithServiceAccount(sa string) *GCPProvider {
	p.ServiceAccount = sa
	return p
}

// AzureProvider manages Azure credentials and connectivity.
type AzureProvider struct {
	SubscriptionID string `json:"subscription_id" yaml:"subscription_id"`
	TenantID       string `json:"tenant_id" yaml:"tenant_id"`
	ClientID       string `json:"client_id,omitempty" yaml:"client_id"`
	ClientSecret   string `json:"client_secret,omitempty" yaml:"client_secret"`
	config         *ProviderConfig
}

// NewAzureProvider creates a new Azure provider with the given credentials.
func NewAzureProvider(
	subscriptionID string,
	tenantID string,
	config *ProviderConfig,
) (*AzureProvider, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("%s", translator.T("storage_provider_azure_subscription_id_required", nil))
	}
	if tenantID == "" {
		return nil, fmt.Errorf("%s", translator.T("storage_provider_azure_tenant_id_required", nil))
	}
	if config == nil {
		config = DefaultProviderConfig()
	}

	return &AzureProvider{
		SubscriptionID: subscriptionID,
		TenantID:       tenantID,
		config:         config,
	}, nil
}

// Name returns "azure".
func (p *AzureProvider) Name() string {
	return "azure"
}

// Credentials returns the Azure credentials.
func (p *AzureProvider) Credentials() map[string]string {
	creds := map[string]string{
		"subscription_id": p.SubscriptionID,
		"tenant_id":       p.TenantID,
	}
	if p.ClientID != "" {
		creds["client_id"] = p.ClientID
	}
	if p.ClientSecret != "" {
		creds["client_secret"] = p.ClientSecret
	}
	return creds
}

// HealthCheck verifies that Azure credentials are configured.
func (p *AzureProvider) HealthCheck(_ context.Context) error {
	if p.SubscriptionID == "" || p.TenantID == "" {
		return fmt.Errorf("%s", translator.T("storage_provider_azure_credentials_not_configured", nil))
	}
	return nil
}

// WithClientCredentials sets service principal credentials.
func (p *AzureProvider) WithClientCredentials(
	clientID string,
	clientSecret string,
) *AzureProvider {
	p.ClientID = clientID
	p.ClientSecret = clientSecret
	return p
}

// Compile-time interface compliance checks.
var (
	_ CloudProvider = (*AWSProvider)(nil)
	_ CloudProvider = (*GCPProvider)(nil)
	_ CloudProvider = (*AzureProvider)(nil)
)
