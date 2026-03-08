#!/usr/bin/env bash
# storage_functionality_challenge.sh - Validates Storage module core functionality
# Checks S3, local filesystem, object store interfaces, and cloud providers
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MODULE_NAME="Storage"

PASS=0
FAIL=0
TOTAL=0

pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

echo "=== ${MODULE_NAME} Functionality Challenge ==="
echo ""

# --- Section 1: Required packages ---
echo "Section 1: Required packages (4)"

for pkg in local object provider s3; do
    echo "Test: Package pkg/${pkg} exists"
    if [ -d "${MODULE_DIR}/pkg/${pkg}" ]; then
        pass "Package pkg/${pkg} exists"
    else
        fail "Package pkg/${pkg} missing"
    fi
done

# --- Section 2: Object store interfaces ---
echo ""
echo "Section 2: Object store interfaces"

echo "Test: ObjectStore interface exists"
if grep -q "type ObjectStore interface" "${MODULE_DIR}/pkg/object/"*.go 2>/dev/null; then
    pass "ObjectStore interface exists"
else
    fail "ObjectStore interface missing"
fi

echo "Test: BucketManager interface exists"
if grep -q "type BucketManager interface" "${MODULE_DIR}/pkg/object/"*.go 2>/dev/null; then
    pass "BucketManager interface exists"
else
    fail "BucketManager interface missing"
fi

echo "Test: ObjectRef struct exists"
if grep -q "type ObjectRef struct" "${MODULE_DIR}/pkg/object/"*.go 2>/dev/null; then
    pass "ObjectRef struct exists"
else
    fail "ObjectRef struct missing"
fi

echo "Test: ObjectInfo struct exists"
if grep -q "type ObjectInfo struct" "${MODULE_DIR}/pkg/object/"*.go 2>/dev/null; then
    pass "ObjectInfo struct exists"
else
    fail "ObjectInfo struct missing"
fi

# --- Section 3: S3/MinIO client ---
echo ""
echo "Section 3: S3/MinIO client"

echo "Test: S3 Client struct exists"
if grep -q "type Client struct" "${MODULE_DIR}/pkg/s3/"*.go 2>/dev/null; then
    pass "S3 Client struct exists"
else
    fail "S3 Client struct missing"
fi

echo "Test: S3 Config struct exists"
if grep -q "type Config struct" "${MODULE_DIR}/pkg/s3/"*.go 2>/dev/null; then
    pass "S3 Config struct exists"
else
    fail "S3 Config struct missing"
fi

echo "Test: MinioClient interface exists"
if grep -q "type MinioClient interface" "${MODULE_DIR}/pkg/s3/"*.go 2>/dev/null; then
    pass "MinioClient interface exists"
else
    fail "MinioClient interface missing"
fi

# --- Section 4: Local filesystem ---
echo ""
echo "Section 4: Local filesystem"

echo "Test: Local Client struct exists"
if grep -q "type Client struct" "${MODULE_DIR}/pkg/local/"*.go 2>/dev/null; then
    pass "Local Client struct exists"
else
    fail "Local Client struct missing"
fi

echo "Test: Local Config struct exists"
if grep -q "type Config struct" "${MODULE_DIR}/pkg/local/"*.go 2>/dev/null; then
    pass "Local Config struct exists"
else
    fail "Local Config struct missing"
fi

# --- Section 5: Cloud providers ---
echo ""
echo "Section 5: Cloud providers"

echo "Test: CloudProvider interface exists"
if grep -q "type CloudProvider interface" "${MODULE_DIR}/pkg/provider/"*.go 2>/dev/null; then
    pass "CloudProvider interface exists"
else
    fail "CloudProvider interface missing"
fi

echo "Test: AWSProvider struct exists"
if grep -q "type AWSProvider struct" "${MODULE_DIR}/pkg/provider/"*.go 2>/dev/null; then
    pass "AWSProvider struct exists"
else
    fail "AWSProvider struct missing"
fi

echo "Test: GCPProvider struct exists"
if grep -q "type GCPProvider struct" "${MODULE_DIR}/pkg/provider/"*.go 2>/dev/null; then
    pass "GCPProvider struct exists"
else
    fail "GCPProvider struct missing"
fi

echo "Test: AzureProvider struct exists"
if grep -q "type AzureProvider struct" "${MODULE_DIR}/pkg/provider/"*.go 2>/dev/null; then
    pass "AzureProvider struct exists"
else
    fail "AzureProvider struct missing"
fi

# --- Section 6: Source structure completeness ---
echo ""
echo "Section 6: Source structure"

echo "Test: Each package has non-test Go source files"
all_have_source=true
for pkg in local object provider s3; do
    non_test=$(find "${MODULE_DIR}/pkg/${pkg}" -name "*.go" ! -name "*_test.go" -type f 2>/dev/null | wc -l)
    if [ "$non_test" -eq 0 ]; then
        fail "Package pkg/${pkg} has no non-test Go files"
        all_have_source=false
    fi
done
if [ "$all_have_source" = true ]; then
    pass "All packages have non-test Go source files"
fi

echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="
[ "${FAIL}" -eq 0 ] && exit 0 || exit 1
