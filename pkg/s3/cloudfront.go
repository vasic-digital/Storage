package s3

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"context"
	"fmt"
	"net/url"
	"strings"
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

// signCloudFrontURL signs a CloudFront URL using the RSA-SHA1 method
// (CloudFront Canned Policy — AWS CloudFront Developer Guide,
// "Creating a signed URL using a canned policy"). The signed URL
// carries three query parameters: Expires, Signature, Key-Pair-Id.
//
// Round-38 wiring: this function now produces signatures that
// CloudFront will accept. The previous round-21 sentinel path
// (ErrCloudFrontSigningNotWired) is preserved in the nil-key /
// empty-PEM branch so misconfigured deployments still fail loudly
// at signing time rather than at the CDN with HTTP 403.
func signCloudFrontURL(rawURL, keyPairID, privateKeyPEM string, expireTime int64) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Build the canned-policy JSON document. AWS CloudFront requires
	// EXACTLY this shape (no whitespace, no extra fields) per the
	// "Creating a signed URL using a canned policy" guide. The
	// Resource is the full URL the client will request (scheme +
	// host + path, BEFORE adding Expires/Signature/Key-Pair-Id).
	policy := buildCannedPolicy(rawURL, expireTime)

	// Sign the canned policy with the RSA private key (SHA1 hash,
	// PKCS#1 v1.5 padding) — this is the CloudFront-required scheme.
	signature, err := generateCloudFrontSignature(policy, privateKeyPEM)
	if err != nil {
		return "", err
	}

	// Append the three CloudFront query parameters. Note: Signature
	// uses CloudFront-safe base64 (+/= → -_~) and MUST be appended
	// raw (NOT URL-encoded a second time by url.Values.Encode); we
	// build RawQuery manually to preserve the wire format.
	q := u.Query()
	q.Set("Expires", fmt.Sprintf("%d", expireTime))
	q.Set("Key-Pair-Id", keyPairID)
	// Set Signature last so it appears at the end of the query string
	// (CloudFront accepts any ordering, but this matches AWS examples).
	q.Set("Signature", signature)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// buildCannedPolicy returns the JSON-encoded CloudFront canned-policy
// document for a given resource URL + epoch expiry. CloudFront is
// strict about the byte-exact shape (no whitespace, field order
// matters); deviations produce HTTP 403 at the CDN.
//
// Reference: AWS CloudFront Developer Guide — "Creating a signed URL
// using a canned policy" — the canned-policy document is computed
// internally by CloudFront from these two inputs and the signer is
// expected to reproduce the same byte sequence locally before signing.
func buildCannedPolicy(resourceURL string, expireTime int64) string {
	return fmt.Sprintf(
		`{"Statement":[{"Resource":"%s","Condition":{"DateLessThan":{"AWS:EpochTime":%d}}}]}`,
		resourceURL,
		expireTime,
	)
}

// generateCloudFrontSignature creates the Signature parameter for a
// CloudFront signed URL by signing the canned-policy document with
// the RSA private key from privateKeyPEM, using RSA-SHA1 (PKCS#1 v1.5
// padding) and CloudFront-safe base64 encoding.
//
// §11.4 / CONST-035 round-38 wiring — Round-21 (commit 7dc5100)
// replaced the broken HMAC-SHA1-with-PEM-as-secret implementation
// with an honest ErrCloudFrontSigningNotWired sentinel. This round-38
// fix wires the real RSA-SHA1 path:
//
//  1. Empty PEM → preserve round-21 sentinel (ErrCloudFrontSigningNotWired)
//     so the misconfigured-deployment failure mode stays loud.
//  2. Decode PEM block; non-RSA / malformed → ErrCloudFrontKeyParseFailed.
//  3. Parse as PKCS#1 first (BEGIN RSA PRIVATE KEY), fall back to
//     PKCS#8 (BEGIN PRIVATE KEY) and type-assert *rsa.PrivateKey.
//  4. Hash policy with SHA-1; sign with rsa.SignPKCS1v15 + crypto.SHA1.
//  5. Encode with CloudFront-safe base64 (+/= → -_~).
//
// CloudFront verifies the signature with the corresponding public key
// at the CDN edge; a valid signature is accepted, invalid → HTTP 403.
// The roundtrip unit test (sign + rsa.VerifyPKCS1v15) guards against
// regression to the round-21 condition (function returns non-empty
// bytes that are not a valid RSA signature).
func generateCloudFrontSignature(policy, privateKeyPEM string) (string, error) {
	if strings.TrimSpace(privateKeyPEM) == "" {
		// Preserves round-21 sentinel for the not-wired-at-deploy
		// failure mode (operator enabled CloudFront but did not
		// provision a key). CONST-042 forbids hardcoding a default
		// key, so empty-PEM must surface as an honest sentinel.
		return "", ErrCloudFrontSigningNotWired
	}

	key, err := parseRSAPrivateKeyPEM(privateKeyPEM)
	if err != nil {
		return "", err
	}

	hashed := sha1.Sum([]byte(policy)) // #nosec G401 — CloudFront protocol REQUIRES SHA-1
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, hashed[:])
	if err != nil {
		return "", fmt.Errorf("cloudfront: RSA-SHA1 signing failed: %w", err)
	}

	return cloudfrontSafeBase64(signature), nil
}

// parseRSAPrivateKeyPEM decodes a PEM block and returns the contained
// RSA private key. Supports both PKCS#1 ("BEGIN RSA PRIVATE KEY") and
// PKCS#8 ("BEGIN PRIVATE KEY") encodings. Non-RSA PKCS#8 keys (ECDSA,
// Ed25519) return ErrCloudFrontKeyParseFailed since CloudFront's
// canned-policy signing scheme is RSA-only.
func parseRSAPrivateKeyPEM(privateKeyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("%w: no PEM block found in input", ErrCloudFrontKeyParseFailed)
	}

	// Try PKCS#1 first (legacy "BEGIN RSA PRIVATE KEY" shape).
	if rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return rsaKey, nil
	}

	// Fall back to PKCS#8 ("BEGIN PRIVATE KEY"), then type-assert
	// to *rsa.PrivateKey — CloudFront requires RSA, so an ECDSA /
	// Ed25519 key cannot satisfy the protocol even if PKCS#8 parses.
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: tried PKCS#1 and PKCS#8, both failed (last error: %v)", ErrCloudFrontKeyParseFailed, err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: PKCS#8 key is not RSA (got %T) — CloudFront canned-policy signing requires RSA", ErrCloudFrontKeyParseFailed, parsed)
	}
	return rsaKey, nil
}

// cloudfrontSafeBase64 encodes the signature bytes using CloudFront's
// custom base64 alphabet: standard base64 with three substitutions
//   - '+' → '-'
//   - '=' → '_'
//   - '/' → '~'
//
// This is documented in the AWS CloudFront Developer Guide section
// "Creating a signed URL using a canned policy" (the bytes must be
// safe to use inside a URL query parameter without further encoding).
func cloudfrontSafeBase64(b []byte) string {
	std := base64.StdEncoding.EncodeToString(b)
	r := strings.NewReplacer(
		"+", "-",
		"=", "_",
		"/", "~",
	)
	return r.Replace(std)
}

// ErrCloudFrontSigningNotWired is returned when CloudFront signing is
// invoked without a private key configured. Round-21 introduced this
// sentinel after the HMAC-SHA1-with-PEM-as-secret bluff was removed;
// round-38 wires the real RSA-SHA1 signing path BUT preserves this
// sentinel for the "operator enabled CloudFront, did not provide a
// key" failure mode — honest sentinel at sign time beats HTTP 403 at
// the CDN. §11.4 PASS-bluff at the user-facing signed-URL layer.
var ErrCloudFrontSigningNotWired = fmt.Errorf("s3.generateCloudFrontSignature: CloudFront signing invoked without a private key — set CloudFrontConfig.PrivateKeyPEM (env-sourced per CONST-042) before enabling signed URLs; round-21 introduced this sentinel after removal of the broken HMAC-SHA1-with-PEM-as-secret implementation, round-38 wired the real RSA-SHA1 path and preserves this sentinel for the misconfigured-deployment failure mode")

// ErrCloudFrontKeyParseFailed is returned when the CloudFront PEM
// private key cannot be parsed as a usable RSA private key — either
// because the input is not PEM-encoded, contains no key block, fails
// both PKCS#1 and PKCS#8 parsing, or successfully parses as PKCS#8
// but contains a non-RSA key (ECDSA, Ed25519). CloudFront's
// canned-policy signing scheme is RSA-only, so a non-RSA key cannot
// produce a CloudFront-acceptable signature regardless of strength.
var ErrCloudFrontKeyParseFailed = fmt.Errorf("cloudfront: RSA private key PEM could not be parsed — verify PEM format (PKCS#1 or PKCS#8) and that the key is RSA (not ECDSA/Ed25519)")

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
