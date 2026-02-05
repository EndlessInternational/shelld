#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/bin/shelld"
CONFIG="$SCRIPT_DIR/config/config.test.toml"
TEST_DIR="$SCRIPT_DIR/test/integration"

# colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

cleanup() {
  pkill -9 -f "shelld.*config.test.toml" 2>/dev/null || true
  sleep 0.5
}

trap cleanup EXIT

start_server() {
  cleanup
  "$BIN" --config "$CONFIG" > /dev/null 2>&1 &
  sleep 2
}

echo "=== shelld Tests ==="
echo ""

# build
echo "Building..."
( cd "$SCRIPT_DIR" && go build -o bin/shelld ./cmd/shelld )
echo ""

# unit tests
echo "Running unit tests..."
( cd "$SCRIPT_DIR" && go test ./internal/... -count=1 )
echo ""

# integration tests
echo "Running integration tests..."
TESTS_PASSED=0
TESTS_FAILED=0

for test_script in "$TEST_DIR"/test_*.sh; do
  if [ -f "$test_script" ]; then
    test_name=$(basename "$test_script" .sh | sed 's/test_//')

    start_server

    echo -n "Running $test_name... "
    if bash "$test_script"; then
      echo -e "${GREEN}PASS${NC}"
      ((TESTS_PASSED++))
    else
      echo -e "${RED}FAIL${NC}"
      ((TESTS_FAILED++))
    fi

    cleanup
  fi
done

echo ""
echo "=== Results ==="
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"

if [ "$TESTS_FAILED" -gt 0 ]; then
  exit 1
fi
