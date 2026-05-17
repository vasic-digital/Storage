package s3

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// CloudFrontConfig holds CloudFront distribution configuration for signed URL generation.
type CloudFrontConfig struct {
	// DistributionDomain is the CloudFront distribution domain (e.g., d1234567890.cloudfront.net)
	DistributionDomain string `json:"distribution_domain" yaml:"distribution_domain"`
	// KeyPairID is the CloudFront key pair ID (from AWS CloudFront > [Distribution] > Key management)
	KeyPairID string `json:"key_pair_id" yaml:"key_pair_id"`
	// PrivateKeyPEM is the RSA private key in PEM format for signing CloudFront URLs
	PrivateKeyPEM string `json:"private_key_pem" yaml:"private_key_pem"`
	// Enabled controls whether CloudFront signed URLs are used instead of S3 presigned URLs
	Enabled bool `json:"enabled" yaml:"enabled"`
	// DefaultExpiry is the default expiry duration for signed URLs
	DefaultExpiry time.Duration `json:"default_expiry" yaml:"default_expiry"`
}

// DefaultCloudFrontConfig returns a CloudFrontConfig with sensible defaults.
func DefaultCloudFrontConfig() *CloudFrontConfig {
	return &CloudFrontConfig{
		DistributionDomain: "",
		KeyPairID:         "",
		PrivateKeyPEM:     "",
		Enabled:            false,
		DefaultExpiry:     24 * time.Hour,
	}
}

// Validate checks that required CloudFront configuration fields are set.
func (c *CloudFrontConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.DistributionDomain == "" {
		return fmt.Errorf("cloudfront: distribution_domain is required when enabled")
	}
	if c.KeyPairID == "" {
		return fmt.Errorf("cloudfront: key_pair_id is required when enabled")
	}
	if c.PrivateKeyPEM == "" {
		return fmt.Errorf("cloudfront: private_key_pem is required when enabled")
	}
	return nil
}

// GetCloudFrontSignedURL generates a CloudFront signed URL for downloading an S3 object.
// It uses the CloudFront RSA signing method (custom policy with Canned Policy or Custom Policy).
func (c *Client) GetCloudFrontSignedURL(
	ctx context.Context,
	bucketName string,
	objectName string,
	expiry time.Duration,
) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return "", fmt.Errorf("not connected to S3")
	}
	if c.config.CloudFront == nil || !c.config.CloudFront.Enabled {
		// Fall back to S3 presigned URL if CloudFront is not configured
		return c.GetPresignedURL(ctx, bucketName, objectName, expiry)
	}

	cf := c.config.CloudFront

	// Build the S3 key (bucket/key path for CloudFront origin if using S3 origin)
	// For S3 origin: URL path is /bucketName/objectName
	// For custom origin: URL path depends on origin configuration
	s3Key := objectName
	if bucketName != "" {
		s3Key = bucketName + "/" + objectName
	}

	// Build the full CloudFront URL
	scheme := "https"
	if !c.config.UseSSL {
		scheme = "http"
	}
	fullURL := fmt.Sprintf("%s://%s/%s", scheme, cf.DistributionDomain, s3Key)

	// Parse the URL for signing
	u, err := url.Parse(fullURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse CloudFront URL: %w", err)
	}

	// Calculate expiry time (Unix epoch seconds)
	expireTime := time.Now().Add(expiry).Unix()
	if expireTime == 0 {
		expireTime = time.Now().Add(cf.DefaultExpiry).Unix()
	}

	// Build the CloudFront signed URL using RSA SHA1 (Canned Policy)
	signedURL, err := signCloudFrontURL(u.String(), cf.KeyPairID, cf.PrivateKeyPEM, expireTime)
	if err != nil {
		return "", fmt.Errorf("failed to sign CloudFront URL: %w", err)
	}

	return signedURL, nil
}

// signCloudFrontURL signs a CloudFront URL using the RSA SHA1 method.
// CloudFront uses a specific signing mechanism documented in AWS CloudFront Developer Guide.
func signCloudFrontURL(rawURL, keyPairID, privateKeyPEM string, expireTime int64) (string, error) {
	// For CloudFront, we need to add the Expires and Signature query parameters
	// The signature is computed over the URL path + expires time

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Add Expires parameter
	q := u.Query()
	q.Set("Expires", fmt.Sprintf("%d", expireTime))
	q.Set("Key-Pair-Id", keyPairID)
	u.RawQuery = q.Encode()

	// Build the string to sign: URL path + expires
	// For CloudFront: stringToSign = "{\"Statement\":[{\"Resource\":\"URL\",\"Condition\":{\"DateLessThan\":{\"AWS:EpochTime\":EXPIRES}}]}"
	// Actually, for Canned Policy, the signature is over the expires time only

	// Simplified CloudFront signing using minio signer patterns
	// In production, this would use the full CloudFront RSA signing
	signature, err := generateCloudFrontSignature(rawURL, expireTime, privateKeyPEM)
	if err != nil {
		return "", err
	}

	q.Set("Signature", signature)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// generateCloudFrontSignature creates the Signature parameter for
// CloudFront signed URLs.
//
// §11.4 / CONST-035 CRITICAL — Previously this function used
// HMAC-SHA1 with the PEM-encoded private key bytes as the HMAC
// secret. AWS CloudFront REQUIRES RSA-SHA1 (not HMAC-SHA1) — the
// HMAC output would be rejected by CloudFront with HTTP 403 on
// every signed-URL access. Any user enabling CloudFront signed URLs
// got URLs that look valid but FAIL at the CDN — §11.4 PASS-bluff
// at the user-facing-API layer.
//
// Fix: return ErrCloudFrontSigningNotWired sentinel so callers
// see the gap loudly. Real implementation requires:
//   block, _ := pem.Decode([]byte(privateKeyPEM))
//   key, err := x509.ParsePKCS1PrivateKey(block.Bytes) // or PKCS8
//   if err != nil { return "", err }
//   h := sha1.New()
//   h.Write([]byte(policy))
//   sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, h.Sum(nil))
//   // CloudFront wants base64-url-safe-modified encoding (+/= → -_~)
//   return cloudfrontSafeEncode(sig), nil
// (round-22+ deferral).
func generateCloudFrontSignature(rawURL string, expireTime int64, privateKeyPEM string) (string, error) {
	_ = rawURL
	_ = expireTime
	_ = privateKeyPEM
	return "", ErrCloudFrontSigningNotWired
}

// ErrCloudFrontSigningNotWired is returned by generateCloudFrontSignature
// until the real RSA-SHA1 + PEM-decode signing path is wired. The
// previous HMAC-SHA1 implementation produced signatures that
// CloudFront would reject with HTTP 403 — §11.4 PASS-bluff at the
// user-facing signed-URL layer; honest sentinel-error surfaces the
// gap before users encounter the 403.
var ErrCloudFrontSigningNotWired = fmt.Errorf("s3.generateCloudFrontSignature: real RSA-SHA1 + PEM-decode signing is not wired (was: HMAC-SHA1 with PEM bytes as HMAC secret — produces signatures CloudFront rejects with HTTP 403 — §11.4 PASS-bluff and is now removed); wire crypto/rsa + crypto/x509 PKCS1 / PKCS8 PEM parse + rsa.SignPKCS1v15 + cloudfront-safe base64 encoding to restore")

// GetCloudFrontSignedURLForTenant generates a tenant-isolated CloudFront signed URL.
// It prepends the tenant namespace to the object key for multi-tenant isolation.
func (c *Client) GetCloudFrontSignedURLForTenant(
	ctx context.Context,
	tenantID string,
	bucketName string,
	objectName string,
	expiry time.Duration,
) (string, error) {
	// Tenant isolation: prepend tenant ID to the object key
	tenantKey := fmt.Sprintf("%s/%s", tenantID, objectName)
	return c.GetCloudFrontSignedURL(ctx, bucketName, tenantKey, expiry)
}

// ConfigureCloudFront updates the client's CloudFront configuration.
func (c *Client) ConfigureCloudFront(cfConfig *CloudFrontConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cfConfig == nil {
		cfConfig = DefaultCloudFrontConfig()
	}
	c.config.CloudFront = cfConfig
}
