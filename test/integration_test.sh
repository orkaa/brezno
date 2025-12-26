#!/bin/bash
set -e

# Integration test suite for Brezno
# Main test runner that orchestrates all test modules
#
# Usage:
#   ./integration_test.sh              # Run all tests
#   ./integration_test.sh basic        # Run only basic tests
#   ./integration_test.sh resize       # Run only resize tests
#   ./integration_test.sh password     # Run only password tests

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BINARY="$PROJECT_DIR/brezno"
TEST_DIR="/tmp/brezno-test-$$"
TEST_CONTAINER="$TEST_DIR/test-container.img"
TEST_MOUNT="$TEST_DIR/mount"
TEST_KEYFILE="$TEST_DIR/test.key"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Test data (shared across test modules)
TEST_DATA=""

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"

    # Try to unmount if mounted
    if mountpoint -q "$TEST_MOUNT" 2>/dev/null; then
        sudo "$BINARY" unmount "$TEST_MOUNT" 2>/dev/null || true
    fi

    # Remove test directory
    rm -rf "$TEST_DIR"

    echo -e "${YELLOW}Cleanup complete${NC}"
}

# Set up cleanup trap
trap cleanup EXIT

# Print test header
print_test() {
    TESTS_RUN=$((TESTS_RUN + 1))
    echo -e "\n${YELLOW}[TEST]${NC} $1"
}

# Print success
print_success() {
    TESTS_PASSED=$((TESTS_PASSED + 1))
    echo -e "${GREEN}✓ PASS${NC} $1"
}

# Print failure and exit
print_failure() {
    TESTS_FAILED=$((TESTS_FAILED + 1))
    echo -e "${RED}✗ FAIL${NC} $1"
    exit 1
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        echo -e "${RED}Error: This test suite must be run as root (use sudo)${NC}"
        exit 1
    fi
}

# Check if binary exists
check_binary() {
    if [ ! -f "$BINARY" ]; then
        echo -e "${RED}Error: Binary not found at $BINARY${NC}"
        echo "Please build the binary first: go build -o brezno ./cmd/brezno"
        exit 1
    fi
}

# Main test execution
main() {
    local TEST_MODULE="${1:-all}"

    echo "========================================"
    echo "  Brezno Integration Test Suite"
    echo "========================================"
    echo ""
    echo "Binary: $BINARY"
    echo "Test directory: $TEST_DIR"
    echo "Mode: $TEST_MODULE"
    echo ""

    check_root
    check_binary

    # Create test directory
    mkdir -p "$TEST_DIR"
    mkdir -p "$TEST_MOUNT"

    # Run requested test modules
    case "$TEST_MODULE" in
        basic)
            echo -e "${YELLOW}Running basic operations tests only...${NC}"
            source "$SCRIPT_DIR/tests/test_basic.sh"
            run_basic_tests
            ;;
        resize)
            echo -e "${YELLOW}Running resize tests only...${NC}"
            source "$SCRIPT_DIR/tests/test_resize.sh"
            run_resize_tests
            ;;
        password)
            echo -e "${YELLOW}Running password tests only...${NC}"
            source "$SCRIPT_DIR/tests/test_password.sh"
            run_password_tests
            ;;
        all)
            echo -e "${YELLOW}Running basic operations tests...${NC}"
            source "$SCRIPT_DIR/tests/test_basic.sh"
            run_basic_tests

            echo -e "\n${YELLOW}Running resize tests...${NC}"
            source "$SCRIPT_DIR/tests/test_resize.sh"
            run_resize_tests

            echo -e "\n${YELLOW}Running password tests...${NC}"
            source "$SCRIPT_DIR/tests/test_password.sh"
            run_password_tests
            ;;
        *)
            echo -e "${RED}Unknown test module: $TEST_MODULE${NC}"
            echo "Usage: $0 [all|basic|resize|password]"
            exit 1
            ;;
    esac

    # Print summary
    echo ""
    echo "========================================"
    echo "  Test Summary"
    echo "========================================"
    echo -e "Total tests: $TESTS_RUN"
    echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    echo ""

    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed!${NC}"
        exit 1
    fi
}

main "$@"
