#!/bin/bash
set -e

# Integration Test Runner for go-transbank-sdk
# This script runs integration tests against the Transbank API

echo "════════════════════════════════════════════════════════════════"
echo "   Transbank Oneclick Integration Test Suite"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Get the SDK directory (where this script is located)
SDK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "SDK Directory: $SDK_DIR"

# Check if we're in the SDK root
if [ ! -f "$SDK_DIR/go.mod" ]; then
  echo "❌ Error: go.mod not found in $SDK_DIR"
  echo "Please run this script from the SDK root directory"
  exit 1
fi

echo ""
echo "Available Test Functions:"
echo "  • TestIntegrationCredentials - Validates credentials"
echo "  • TestIntegrationInscriptionFlow - Complete inscription flow"
echo "  • TestIntegrationAuthorizeTransaction - Authorize & check status"
echo "  • TestIntegrationTransactionStatus - Query transaction status"
echo "  • TestIntegrationRefundTransaction - Authorize & refund"
echo "  • TestIntegrationDeleteInscription - Create & delete inscription"
echo ""

# Parse arguments
TEST_NAME="${1:-}" # Optional: specific test to run
VERBOSE="${2:-}" # Optional: -v for verbose

# Build test command
TEST_CMD="go test -tags=integration -v"

if [ -n "$TEST_NAME" ]; then
  TEST_CMD="$TEST_CMD -run $TEST_NAME"
fi

if [ "$VERBOSE" = "-v" ]; then
  TEST_CMD="$TEST_CMD -v"
fi

TEST_CMD="$TEST_CMD ./oneclick"

echo "Environment Variables (optional overrides):"
echo "  TRANSBANK_COMMERCE_CODE    (default: 597055555541)"
echo "  TRANSBANK_API_SECRET        (default: 579B532A7440BB...)"
echo "  TRANSBANK_BASE_URL          (default: https://webpay3gint...)"
echo "  TRANSBANK_TEST_TBK_USER     (default: b6bd6ba3-e718...)"
echo ""

echo "Running: $TEST_CMD"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Run the tests
cd "$SDK_DIR"
export TRANSBANK_COMMERCE_CODE="${TRANSBANK_COMMERCE_CODE:-597055555541}"
export TRANSBANK_API_SECRET="${TRANSBANK_API_SECRET:-579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C}"
export TRANSBANK_BASE_URL="${TRANSBANK_BASE_URL:-https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2}"
export TRANSBANK_TEST_TBK_USER="${TRANSBANK_TEST_TBK_USER:-b6bd6ba3-e718-4107-9386-d2b099a8dd42}"

$TEST_CMD

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "✓ Integration tests completed successfully!"
echo "════════════════════════════════════════════════════════════════"
