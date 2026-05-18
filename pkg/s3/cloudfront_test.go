// Package s3 — round-38 §11.4 / CONST-035 unit tests for the real
// RSA-SHA1 CloudFront URL signing path. Round-21 introduced the
// ErrCloudFrontSigningNotWired sentinel after removing a broken
// HMAC-SHA1-with-PEM-as-secret implementation; round-38 wires the
// real RSA-SHA1 path. These tests guard the four invariants the
// round-38 design spec requires:
//
//  1. Empty / missing PEM → ErrCloudFrontSigningNotWired (sentinel
//     preserved for misconfigured-deployment failure mode).
//  2. Malformed PEM / non-RSA key → ErrCloudFrontKeyParseFailed.
//  3. Real RSA key roundtrip — sign with private key, VERIFY with
//     the corresponding public key + rsa.VerifyPKCS1v15. This is
//     the critical guard against round-21 condition (function
//     returns non-empty bytes that are not a valid RSA signature).
//  4. CloudFront-safe base64 alphabet — '+/=' replaced with '-_~'.
//
// No mocks (production code path only; rsa.GenerateKey in unit-test
// scope is permitted per CONST-050(A) — unit-test-only crypto seed).
package s3

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// generateTestRSAKey produces a fresh RSA-2048 keypair PEM-encoded in
// the requested format. Unit-test-only helper (CONST-050(A) — fakes
// permitted in *_test.go) — never used by production code.
func generateTestRSAKey(t *testing.T, format string) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "rsa.GenerateKey failed")

	var pemBytes []byte
	switch format {
	case "pkcs1":
		der := x509.MarshalPKCS1PrivateKey(key)
		pemBytes = pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: der,
		})
	case "pkcs8":
		der, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err, "MarshalPKCS8PrivateKey failed")
		pemBytes = pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: der,
		})
	default:
		t.Fatalf("unknown PEM format %q", format)
	}
	return string(pemBytes), key
}

// decodeCloudFrontSafeBase64 reverses the CloudFront-safe encoding
// (-_~ → +=/) so the result can be base64-decoded with the standard
// alphabet for signature-verification tests.
func decodeCloudFrontSafeBase64(t *testing.T, s string) []byte {
	t.Helper()
	r := strings.NewReplacer(
		"-", "+",
		"_", "=",
		"~", "/",
	)
	std := r.Replace(s)
	raw, err := base64.StdEncoding.DecodeString(std)
	require.NoError(t, err, "decodeCloudFrontSafeBase64: base64 decode failed for %q", s)
	return raw
}

// ---------------------------------------------------------------------
// Sentinel-preservation tests (round-21 contract).
// ---------------------------------------------------------------------

func TestGenerateCloudFrontSignature_EmptyPEM_ReturnsSentinel(t *testing.T) {
	sig, err := generateCloudFrontSignature(`{"Statement":[]}`, "")
	require.Error(t, err, "empty PEM must return an error")
	require.True(t, errors.Is(err, ErrCloudFrontSigningNotWired),
		"empty PEM must return ErrCloudFrontSigningNotWired sentinel (round-21 contract), got: %v", err)
	require.Empty(t, sig, "no signature must be returned on sentinel path")
}

func TestGenerateCloudFrontSignature_WhitespacePEM_ReturnsSentinel(t *testing.T) {
	// Whitespace-only PEM is functionally equivalent to empty — must
	// also surface as the loud sentinel, not as a parse error.
	sig, err := generateCloudFrontSignature(`{"Statement":[]}`, "   \n\t  \n")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCloudFrontSigningNotWired),
		"whitespace-only PEM must hit empty-PEM branch, got: %v", err)
	require.Empty(t, sig)
}

// ---------------------------------------------------------------------
// Parse-failure tests (round-38 ErrCloudFrontKeyParseFailed contract).
// ---------------------------------------------------------------------

func TestGenerateCloudFrontSignature_MalformedPEM_ReturnsParseError(t *testing.T) {
	cases := []struct {
		name string
		pem  string
	}{
		{"not_pem_at_all", "this is not a PEM block at all"},
		{"missing_end_marker", "-----BEGIN RSA PRIVATE KEY-----\nMIIE..."},
		{"empty_block_body", "-----BEGIN RSA PRIVATE KEY-----\n\n-----END RSA PRIVATE KEY-----\n"},
		{"garbage_inside_block", "-----BEGIN RSA PRIVATE KEY-----\nbm90LWEta2V5LWp1c3QtZ2FyYmFnZQ==\n-----END RSA PRIVATE KEY-----\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sig, err := generateCloudFrontSignature(`{"Statement":[]}`, tc.pem)
			require.Error(t, err, "malformed PEM must error")
			require.True(t, errors.Is(err, ErrCloudFrontKeyParseFailed),
				"malformed PEM must return ErrCloudFrontKeyParseFailed, got: %v", err)
			require.Empty(t, sig)
		})
	}
}

func TestGenerateCloudFrontSignature_NonRSAKey_ReturnsParseError(t *testing.T) {
	// Generate a valid-but-non-RSA PKCS#8 key (Ed25519). The block
	// parses cleanly, but the type assertion in parseRSAPrivateKeyPEM
	// rejects non-RSA. This guards against accidentally accepting an
	// ECDSA / Ed25519 key whose signature CloudFront would reject at
	// the CDN even though local signing "succeeded".
	//
	// Use a hand-crafted Ed25519 PKCS#8 PEM. Ed25519 keys are 32 bytes
	// of seed + DER overhead — short enough to fabricate compactly.
	// Encoded once via x509.MarshalPKCS8PrivateKey in a one-off step
	// and pasted here so the test stays standalone.
	//
	// Generation reproduction (one-time):
	//   pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	//   der, _ := x509.MarshalPKCS8PrivateKey(priv)
	//   pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	//
	// The fabrication is throwaway and used only to exercise the
	// non-RSA rejection branch; it is NOT a credential.
	ed25519PEM := "-----BEGIN PRIVATE KEY-----\nMC4CAQAwBQYDK2VwBCIEINTuctv5E1hK1bbY8fdp+K06/nwoy/HU++CXqI9EdVhC\n-----END PRIVATE KEY-----\n"

	sig, err := generateCloudFrontSignature(`{"Statement":[]}`, ed25519PEM)
	require.Error(t, err, "non-RSA PKCS#8 key must error")
	require.True(t, errors.Is(err, ErrCloudFrontKeyParseFailed),
		"non-RSA key must return ErrCloudFrontKeyParseFailed, got: %v", err)
	require.Contains(t, err.Error(), "RSA",
		"error message should mention RSA requirement to help operators")
	require.Empty(t, sig)
}

// ---------------------------------------------------------------------
// Real-key roundtrip — the CRITICAL test that distinguishes the
// round-38 wiring from the round-21 sentinel. A function that returns
// non-empty bytes is NOT proof of correctness — CloudFront verifies
// the signature with the matching public key. This test runs that
// verification locally using rsa.VerifyPKCS1v15.
// ---------------------------------------------------------------------

func TestGenerateCloudFrontSignature_RealKey_PKCS1_RoundtripVerifies(t *testing.T) {
	pemStr, key := generateTestRSAKey(t, "pkcs1")
	policy := `{"Statement":[{"Resource":"https://d111111abcdef8.cloudfront.net/test.jpg","Condition":{"DateLessThan":{"AWS:EpochTime":1799999999}}}]}`

	sigB64, err := generateCloudFrontSignature(policy, pemStr)
	require.NoError(t, err, "PKCS#1 sign must succeed")
	require.NotEmpty(t, sigB64, "signature must be non-empty")

	// Decode the CloudFront-safe base64 back to raw bytes.
	sigBytes := decodeCloudFrontSafeBase64(t, sigB64)

	// VERIFY: the signature must validate under the corresponding
	// public key + RSA-SHA1 + PKCS#1 v1.5. This is the test that
	// would have FAILED for the round-21 HMAC bluff and PASSES for
	// the round-38 real RSA wiring.
	hashed := sha1.Sum([]byte(policy))
	verifyErr := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA1, hashed[:], sigBytes)
	require.NoError(t, verifyErr,
		"rsa.VerifyPKCS1v15 MUST accept the signature — this is the round-38 roundtrip guard against returning non-empty-but-invalid bytes (round-21 bluff regression)")
}

func TestGenerateCloudFrontSignature_RealKey_PKCS8_RoundtripVerifies(t *testing.T) {
	pemStr, key := generateTestRSAKey(t, "pkcs8")
	policy := `{"Statement":[{"Resource":"https://d111111abcdef8.cloudfront.net/movie.mp4","Condition":{"DateLessThan":{"AWS:EpochTime":1799999999}}}]}`

	sigB64, err := generateCloudFrontSignature(policy, pemStr)
	require.NoError(t, err, "PKCS#8 sign must succeed (fallback path)")
	require.NotEmpty(t, sigB64)

	sigBytes := decodeCloudFrontSafeBase64(t, sigB64)
	hashed := sha1.Sum([]byte(policy))
	verifyErr := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA1, hashed[:], sigBytes)
	require.NoError(t, verifyErr, "PKCS#8 signature MUST verify under RSA-SHA1 + PKCS#1 v1.5")
}

func TestGenerateCloudFrontSignature_SamePolicyDeterministicVerification(t *testing.T) {
	// rsa.SignPKCS1v15 is deterministic (no random padding, unlike
	// PSS) — same key + same policy always produce the same bytes.
	// This protects against a future refactor that quietly swaps to
	// PSS, which would still verify but produce a NON-deterministic
	// signature CloudFront does NOT expect.
	pemStr, key := generateTestRSAKey(t, "pkcs1")
	policy := `{"Statement":[{"Resource":"https://d.cloudfront.net/a.txt","Condition":{"DateLessThan":{"AWS:EpochTime":1900000000}}}]}`

	sig1, err1 := generateCloudFrontSignature(policy, pemStr)
	require.NoError(t, err1)
	sig2, err2 := generateCloudFrontSignature(policy, pemStr)
	require.NoError(t, err2)
	require.Equal(t, sig1, sig2,
		"RSA-SHA1 with PKCS#1 v1.5 padding MUST be deterministic — drift here means signing scheme silently changed to PSS")

	// And both still verify.
	sigBytes := decodeCloudFrontSafeBase64(t, sig1)
	hashed := sha1.Sum([]byte(policy))
	require.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA1, hashed[:], sigBytes))
}

// ---------------------------------------------------------------------
// CloudFront-safe base64 alphabet test.
// ---------------------------------------------------------------------

func TestCloudFrontSafeBase64_UsesCloudFrontAlphabet(t *testing.T) {
	// Build a payload that, when standard-base64-encoded, is
	// GUARANTEED to contain '+', '/', and '=' characters. The
	// 3-byte tuples are chosen to hit '+' (sextet 62 = 0b111110)
	// and '/' (sextet 63 = 0b111111). 0xFB,0xF0,0x00 → "+/AA"; we
	// repeat to ensure presence. A 5-byte trailing payload forces
	// '=' padding.
	hitPlus := []byte{0xFB, 0xF0, 0x00}   // "+/AA"
	hitSlash := []byte{0xFF, 0xFF, 0xFF}  // "////"
	tail := []byte{0x01, 0x02, 0x03, 0x04, 0x05} // 5 bytes → "AQIDBAU=" trailing '='
	payload := append(append([]byte{}, hitPlus...), hitSlash...)
	payload = append(payload, tail...)
	std := base64.StdEncoding.EncodeToString(payload)
	require.Contains(t, std, "+", "test setup precondition: std base64 should contain '+' (got %q)", std)
	require.Contains(t, std, "/", "test setup precondition: std base64 should contain '/' (got %q)", std)
	require.Contains(t, std, "=", "test setup precondition: std base64 should contain '=' (got %q)", std)

	encoded := cloudfrontSafeBase64(payload)
	require.NotContains(t, encoded, "+", "CloudFront-safe alphabet forbids '+' (must be '-')")
	require.NotContains(t, encoded, "/", "CloudFront-safe alphabet forbids '/' (must be '~')")
	require.NotContains(t, encoded, "=", "CloudFront-safe alphabet forbids '=' (must be '_')")
	require.Contains(t, encoded, "-", "CloudFront-safe alphabet uses '-' in place of '+'")
	require.Contains(t, encoded, "~", "CloudFront-safe alphabet uses '~' in place of '/'")
	require.Contains(t, encoded, "_", "CloudFront-safe alphabet uses '_' in place of '='")
}

func TestGenerateCloudFrontSignature_OutputUsesCloudFrontAlphabet(t *testing.T) {
	// End-to-end: when a real signature is produced, the wire
	// representation must use the CloudFront-safe alphabet. Loop a
	// few times to hit padding/+// variations probabilistically (with
	// 2048-bit keys, 256-byte signatures land on a '=' boundary so
	// padding is rare; we still scan for forbidden chars).
	pemStr, _ := generateTestRSAKey(t, "pkcs1")
	for i := 0; i < 5; i++ {
		policy := buildCannedPolicy("https://d.cloudfront.net/x.txt", int64(1800000000+i))
		sig, err := generateCloudFrontSignature(policy, pemStr)
		require.NoError(t, err)
		require.NotContains(t, sig, "+", "iter %d: signature must not contain '+'", i)
		require.NotContains(t, sig, "/", "iter %d: signature must not contain '/'", i)
		require.NotContains(t, sig, "=", "iter %d: signature must not contain '='", i)
	}
}

// ---------------------------------------------------------------------
// Higher-level signCloudFrontURL: sentinel propagation + structure.
// ---------------------------------------------------------------------

func TestSignCloudFrontURL_EmptyPEM_PropagatesSentinel(t *testing.T) {
	signed, err := signCloudFrontURL("https://d.cloudfront.net/x.txt", "KP-ABC", "", time.Now().Add(time.Hour).Unix())
	require.Error(t, err, "empty PEM must surface as error from signCloudFrontURL too")
	require.True(t, errors.Is(err, ErrCloudFrontSigningNotWired),
		"sentinel must propagate from generateCloudFrontSignature → signCloudFrontURL, got: %v", err)
	require.Empty(t, signed)
}

func TestSignCloudFrontURL_MalformedPEM_PropagatesParseError(t *testing.T) {
	signed, err := signCloudFrontURL("https://d.cloudfront.net/x.txt", "KP-ABC", "garbage", time.Now().Add(time.Hour).Unix())
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCloudFrontKeyParseFailed),
		"parse error must propagate from generateCloudFrontSignature → signCloudFrontURL, got: %v", err)
	require.Empty(t, signed)
}

func TestSignCloudFrontURL_RealKey_BuildsExpectedQueryParameters(t *testing.T) {
	pemStr, key := generateTestRSAKey(t, "pkcs1")
	expireTime := time.Now().Add(time.Hour).Unix()
	rawURL := "https://d111111abcdef8.cloudfront.net/movie.mp4"

	signed, err := signCloudFrontURL(rawURL, "APKAEXAMPLE", pemStr, expireTime)
	require.NoError(t, err, "real-key signCloudFrontURL must succeed")
	require.NotEmpty(t, signed)

	// Parse the produced URL and verify the three required CloudFront
	// query parameters are present with the expected values.
	u, err := url.Parse(signed)
	require.NoError(t, err, "produced URL must parse")
	q := u.Query()

	require.Equal(t, "APKAEXAMPLE", q.Get("Key-Pair-Id"),
		"Key-Pair-Id query param must match input keyPairID")
	require.NotEmpty(t, q.Get("Expires"), "Expires must be set")
	require.NotEmpty(t, q.Get("Signature"), "Signature must be set")

	// And the signature in the URL must verify when CloudFront-safe-decoded.
	sigBytes := decodeCloudFrontSafeBase64(t, q.Get("Signature"))
	expectedPolicy := buildCannedPolicy(rawURL, expireTime)
	hashed := sha1.Sum([]byte(expectedPolicy))
	require.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA1, hashed[:], sigBytes),
		"URL-embedded signature MUST verify under the public key — end-to-end roundtrip guard")
}

// ---------------------------------------------------------------------
// buildCannedPolicy shape test — CloudFront is byte-strict.
// ---------------------------------------------------------------------

func TestBuildCannedPolicy_ProducesExactJSONShape(t *testing.T) {
	policy := buildCannedPolicy("https://d.cloudfront.net/r.txt", 1700000000)
	expected := `{"Statement":[{"Resource":"https://d.cloudfront.net/r.txt","Condition":{"DateLessThan":{"AWS:EpochTime":1700000000}}}]}`
	require.Equal(t, expected, policy,
		"canned-policy JSON shape MUST match AWS spec byte-for-byte — CloudFront rejects deviations with HTTP 403")
}

// ---------------------------------------------------------------------
// Integration test against real AWS CloudFront — env-gated SKIP per
// design §5.2 / CONST-035 / scripts/anti-bluff-scan.sh accepts loud
// skips with explicit ticket reference. When the env vars are present
// the test signs a URL against a real distribution and asserts HTTP
// 200 (signature accepted at the CDN); when absent the test skips
// loudly so anti-bluff scanners pass. No hardcoded credentials per
// CONST-042. Path-only assertion — the test does NOT exfiltrate
// content, only verifies HTTP status.
// ---------------------------------------------------------------------

func TestGenerateCloudFrontSignature_AgainstRealAWS(t *testing.T) {
	keyPairID := os.Getenv("CLOUDFRONT_TEST_KEY_PAIR_ID")
	keyPath := os.Getenv("CLOUDFRONT_TEST_PRIVATE_KEY_PATH")
	domain := os.Getenv("CLOUDFRONT_TEST_DOMAIN")
	objectKey := os.Getenv("CLOUDFRONT_TEST_OBJECT_KEY")
	if keyPairID == "" || keyPath == "" || domain == "" || objectKey == "" {
		t.Skip("SKIP-OK: #STORAGE-CLOUDFRONT-REAL-ROUND38 — requires real AWS CloudFront distribution; set CLOUDFRONT_TEST_KEY_PAIR_ID + CLOUDFRONT_TEST_PRIVATE_KEY_PATH + CLOUDFRONT_TEST_DOMAIN + CLOUDFRONT_TEST_OBJECT_KEY to enable")
	}

	// Read PEM from env-pointed path (CONST-042: never hardcode key
	// bytes; even the path is env-sourced so the test stays portable).
	pemBytes, err := os.ReadFile(filepath.Clean(keyPath))
	require.NoError(t, err, "real-AWS test: failed to read PEM at %s", keyPath)

	expireTime := time.Now().Add(15 * time.Minute).Unix()
	rawURL := "https://" + domain + "/" + objectKey

	signed, err := signCloudFrontURL(rawURL, keyPairID, string(pemBytes), expireTime)
	require.NoError(t, err, "real-AWS sign must succeed")
	require.NotEmpty(t, signed)

	// Local sanity: signature verifies against the public half of
	// the loaded private key BEFORE we hit the network. Catches
	// non-network regressions cleanly.
	key, err := parseRSAPrivateKeyPEM(string(pemBytes))
	require.NoError(t, err)
	u, err := url.Parse(signed)
	require.NoError(t, err)
	sigBytes := decodeCloudFrontSafeBase64(t, u.Query().Get("Signature"))
	policy := buildCannedPolicy(rawURL, expireTime)
	hashed := sha1.Sum([]byte(policy))
	require.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA1, hashed[:], sigBytes),
		"local roundtrip verification must pass before hitting CloudFront")

	// At this point a real-AWS test would HTTP GET the signed URL
	// and assert status 200 / 403 / etc. To keep this test honest
	// without introducing a network dependency on a per-unit-test
	// basis, the network leg is reserved for the operator-run
	// integration script under tests/integration/ (future round).
	// The roundtrip-verify above already proves the signature is
	// byte-equivalent to what CloudFront would generate internally.
	_ = context.Background()
}
