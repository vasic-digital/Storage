package provider

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ProviderConfig tests ---

func TestDefaultProviderConfig(t *testing.T) {
	config := DefaultProviderConfig()
	assert.Equal(t, 30*time.Second, config.Timeout)
}

// --- AWSProvider tests ---

func TestNewAWSProvider(t *testing.T) {
	tests := []struct {
		name        string
		accessKey   string
		secretKey   string
		region      string
		config      *ProviderConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid credentials",
			accessKey:   "AKIAIOSFODNN7EXAMPLE",
			secretKey:   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			region:      "us-east-1",
			config:      nil,
			expectError: false,
		},
		{
			name:        "with explicit config",
			accessKey:   "AKID",
			secretKey:   "SECRET",
			region:      "eu-west-1",
			config:      &ProviderConfig{Timeout: 60 * time.Second},
			expectError: false,
		},
		{
			name:        "empty access key",
			accessKey:   "",
			secretKey:   "secret",
			region:      "us-east-1",
			expectError: true,
			errorMsg:    "access_key_id is required",
		},
		{
			name:        "empty secret key",
			accessKey:   "access",
			secretKey:   "",
			region:      "us-east-1",
			expectError: true,
			errorMsg:    "secret_access_key is required",
		},
		{
			name:        "empty region",
			accessKey:   "access",
			secretKey:   "secret",
			region:      "",
			expectError: true,
			errorMsg:    "region is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewAWSProvider(
				tt.accessKey, tt.secretKey, tt.region, tt.config,
			)
			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, p)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, p)
			}
		})
	}
}

func TestAWSProvider_Name(t *testing.T) {
	p, err := NewAWSProvider("ak", "sk", "us-east-1", nil)
	require.NoError(t, err)
	assert.Equal(t, "aws", p.Name())
}

func TestAWSProvider_Credentials(t *testing.T) {
	tests := []struct {
		name         string
		sessionToken string
		expectKeys   []string
	}{
		{
			name:         "without session token",
			sessionToken: "",
			expectKeys:   []string{"access_key_id", "secret_access_key", "region"},
		},
		{
			name:         "with session token",
			sessionToken: "token123",
			expectKeys: []string{
				"access_key_id", "secret_access_key",
				"region", "session_token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewAWSProvider("ak", "sk", "us-east-1", nil)
			require.NoError(t, err)
			if tt.sessionToken != "" {
				p.WithSessionToken(tt.sessionToken)
			}

			creds := p.Credentials()
			for _, key := range tt.expectKeys {
				assert.Contains(t, creds, key)
			}
			assert.Equal(t, "ak", creds["access_key_id"])
			assert.Equal(t, "sk", creds["secret_access_key"])
			assert.Equal(t, "us-east-1", creds["region"])
		})
	}
}

func TestAWSProvider_HealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		accessKey   string
		secretKey   string
		expectError bool
	}{
		{"valid", "ak", "sk", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewAWSProvider(tt.accessKey, tt.secretKey, "us-east-1", nil)
			require.NoError(t, err)
			err = p.HealthCheck(context.Background())
			if tt.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAWSProvider_WithSessionToken(t *testing.T) {
	p, err := NewAWSProvider("ak", "sk", "us-east-1", nil)
	require.NoError(t, err)

	result := p.WithSessionToken("sess-token")
	assert.Equal(t, p, result) // Returns self for chaining
	assert.Equal(t, "sess-token", p.SessionToken)
}

// --- GCPProvider tests ---

func TestNewGCPProvider(t *testing.T) {
	tests := []struct {
		name        string
		projectID   string
		location    string
		config      *ProviderConfig
		expectError bool
		errorMsg    string
		expectLoc   string
	}{
		{
			name:        "valid with explicit location",
			projectID:   "my-project",
			location:    "europe-west1",
			config:      nil,
			expectError: false,
			expectLoc:   "europe-west1",
		},
		{
			name:        "valid with default location",
			projectID:   "my-project",
			location:    "",
			config:      nil,
			expectError: false,
			expectLoc:   "us-central1",
		},
		{
			name:        "empty project id",
			projectID:   "",
			location:    "us-central1",
			expectError: true,
			errorMsg:    "project_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewGCPProvider(tt.projectID, tt.location, tt.config)
			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, p)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, p)
				assert.Equal(t, tt.expectLoc, p.Location)
			}
		})
	}
}

func TestGCPProvider_Name(t *testing.T) {
	p, err := NewGCPProvider("proj", "us-central1", nil)
	require.NoError(t, err)
	assert.Equal(t, "gcp", p.Name())
}

func TestGCPProvider_Credentials(t *testing.T) {
	tests := []struct {
		name           string
		serviceAccount string
		expectKeys     []string
	}{
		{
			name:           "without service account",
			serviceAccount: "",
			expectKeys:     []string{"project_id", "location"},
		},
		{
			name:           "with service account",
			serviceAccount: "sa@proj.iam.gserviceaccount.com",
			expectKeys:     []string{"project_id", "location", "service_account"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewGCPProvider("proj", "us-central1", nil)
			require.NoError(t, err)
			if tt.serviceAccount != "" {
				p.WithServiceAccount(tt.serviceAccount)
			}

			creds := p.Credentials()
			for _, key := range tt.expectKeys {
				assert.Contains(t, creds, key)
			}
		})
	}
}

func TestGCPProvider_HealthCheck(t *testing.T) {
	p, err := NewGCPProvider("proj", "us-central1", nil)
	require.NoError(t, err)
	err = p.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestGCPProvider_WithServiceAccount(t *testing.T) {
	p, err := NewGCPProvider("proj", "us-central1", nil)
	require.NoError(t, err)

	result := p.WithServiceAccount("sa@test.com")
	assert.Equal(t, p, result)
	assert.Equal(t, "sa@test.com", p.ServiceAccount)
}

// --- AzureProvider tests ---

func TestNewAzureProvider(t *testing.T) {
	tests := []struct {
		name           string
		subscriptionID string
		tenantID       string
		config         *ProviderConfig
		expectError    bool
		errorMsg       string
	}{
		{
			name:           "valid",
			subscriptionID: "sub-123",
			tenantID:       "tenant-456",
			config:         nil,
			expectError:    false,
		},
		{
			name:           "empty subscription",
			subscriptionID: "",
			tenantID:       "tenant",
			expectError:    true,
			errorMsg:       "subscription_id is required",
		},
		{
			name:           "empty tenant",
			subscriptionID: "sub",
			tenantID:       "",
			expectError:    true,
			errorMsg:       "tenant_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewAzureProvider(
				tt.subscriptionID, tt.tenantID, tt.config,
			)
			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, p)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, p)
			}
		})
	}
}

func TestAzureProvider_Name(t *testing.T) {
	p, err := NewAzureProvider("sub", "tenant", nil)
	require.NoError(t, err)
	assert.Equal(t, "azure", p.Name())
}

func TestAzureProvider_Credentials(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		expectKeys   []string
	}{
		{
			name:       "without client credentials",
			expectKeys: []string{"subscription_id", "tenant_id"},
		},
		{
			name:         "with client credentials",
			clientID:     "client-id",
			clientSecret: "client-secret",
			expectKeys: []string{
				"subscription_id", "tenant_id",
				"client_id", "client_secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewAzureProvider("sub", "tenant", nil)
			require.NoError(t, err)
			if tt.clientID != "" {
				p.WithClientCredentials(tt.clientID, tt.clientSecret)
			}

			creds := p.Credentials()
			for _, key := range tt.expectKeys {
				assert.Contains(t, creds, key)
			}
		})
	}
}

func TestAzureProvider_HealthCheck(t *testing.T) {
	p, err := NewAzureProvider("sub", "tenant", nil)
	require.NoError(t, err)
	err = p.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestAzureProvider_WithClientCredentials(t *testing.T) {
	p, err := NewAzureProvider("sub", "tenant", nil)
	require.NoError(t, err)

	result := p.WithClientCredentials("cid", "csecret")
	assert.Equal(t, p, result)
	assert.Equal(t, "cid", p.ClientID)
	assert.Equal(t, "csecret", p.ClientSecret)
}

// --- Interface compliance tests ---

func TestCloudProviderInterface(t *testing.T) {
	tests := []struct {
		name     string
		provider CloudProvider
	}{
		{
			name: "AWS provider",
			provider: func() CloudProvider {
				p, _ := NewAWSProvider("ak", "sk", "us-east-1", nil)
				return p
			}(),
		},
		{
			name: "GCP provider",
			provider: func() CloudProvider {
				p, _ := NewGCPProvider("proj", "us-central1", nil)
				return p
			}(),
		},
		{
			name: "Azure provider",
			provider: func() CloudProvider {
				p, _ := NewAzureProvider("sub", "tenant", nil)
				return p
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.provider.Name())
			assert.NotNil(t, tt.provider.Credentials())
			err := tt.provider.HealthCheck(context.Background())
			assert.NoError(t, err)
		})
	}
}

// --- HealthCheck error case tests ---

func TestAWSProvider_HealthCheck_EmptyAccessKey(t *testing.T) {
	// Create a provider with valid credentials first
	p, err := NewAWSProvider("ak", "sk", "us-east-1", nil)
	require.NoError(t, err)

	// Manually set AccessKeyID to empty to trigger error path
	p.AccessKeyID = ""

	err = p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AWS credentials not configured")
}

func TestAWSProvider_HealthCheck_EmptySecretKey(t *testing.T) {
	// Create a provider with valid credentials first
	p, err := NewAWSProvider("ak", "sk", "us-east-1", nil)
	require.NoError(t, err)

	// Manually set SecretAccessKey to empty to trigger error path
	p.SecretAccessKey = ""

	err = p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AWS credentials not configured")
}

func TestAWSProvider_HealthCheck_BothEmpty(t *testing.T) {
	// Create a provider with valid credentials first
	p, err := NewAWSProvider("ak", "sk", "us-east-1", nil)
	require.NoError(t, err)

	// Manually set both to empty
	p.AccessKeyID = ""
	p.SecretAccessKey = ""

	err = p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AWS credentials not configured")
}

func TestGCPProvider_HealthCheck_EmptyProjectID(t *testing.T) {
	// Create a provider with valid credentials first
	p, err := NewGCPProvider("proj", "us-central1", nil)
	require.NoError(t, err)

	// Manually set ProjectID to empty to trigger error path
	p.ProjectID = ""

	err = p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GCP project ID not configured")
}

func TestAzureProvider_HealthCheck_EmptySubscriptionID(t *testing.T) {
	// Create a provider with valid credentials first
	p, err := NewAzureProvider("sub", "tenant", nil)
	require.NoError(t, err)

	// Manually set SubscriptionID to empty to trigger error path
	p.SubscriptionID = ""

	err = p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Azure credentials not configured")
}

func TestAzureProvider_HealthCheck_EmptyTenantID(t *testing.T) {
	// Create a provider with valid credentials first
	p, err := NewAzureProvider("sub", "tenant", nil)
	require.NoError(t, err)

	// Manually set TenantID to empty to trigger error path
	p.TenantID = ""

	err = p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Azure credentials not configured")
}

func TestAzureProvider_HealthCheck_BothEmpty(t *testing.T) {
	// Create a provider with valid credentials first
	p, err := NewAzureProvider("sub", "tenant", nil)
	require.NoError(t, err)

	// Manually set both to empty
	p.SubscriptionID = ""
	p.TenantID = ""

	err = p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Azure credentials not configured")
}
