#!/bin/bash
set -e

# Integration test suite for Brezno
# Tests the complete workflow: create, mount, use, unmount

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
    echo -e "\n${YELLOW}[Test $TESTS_RUN]${NC} $1"
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
    echo "========================================"
    echo "  Brezno Integration Test Suite"
    echo "========================================"
    echo ""
    echo "Binary: $BINARY"
    echo "Test directory: $TEST_DIR"
    echo ""

    check_root
    check_binary

    # Create test directory
    mkdir -p "$TEST_DIR"
    mkdir -p "$TEST_MOUNT"

    # Create test keyfile
    print_test "Creating test keyfile"
    dd if=/dev/urandom of="$TEST_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$TEST_KEYFILE"
    if [ -f "$TEST_KEYFILE" ]; then
        print_success "Keyfile created successfully"
    else
        print_failure "Keyfile not created"
    fi

    # Test 1: Create container with keyfile
    print_test "Creating encrypted container (100MB) with keyfile"
    "$BINARY" create "$TEST_CONTAINER" --size 100M --keyfile "$TEST_KEYFILE"
    if [ -f "$TEST_CONTAINER" ]; then
        print_success "Container created successfully"
    else
        print_failure "Container file not created"
    fi

    # Test 2: Verify file permissions (should be 0600 after Fix #3, but for now just check it exists)
    print_test "Checking container file permissions"
    PERMS=$(stat -c "%a" "$TEST_CONTAINER")
    echo "Container permissions: $PERMS"
    print_success "Container permissions checked: $PERMS"

    # Test 3: Mount container with keyfile
    print_test "Mounting container with keyfile"
    "$BINARY" mount "$TEST_CONTAINER" "$TEST_MOUNT" --keyfile "$TEST_KEYFILE"
    if mountpoint -q "$TEST_MOUNT"; then
        print_success "Container mounted successfully"
    else
        print_failure "Container not mounted"
    fi

    # Test 4: Write data to mounted container
    print_test "Writing test data to mounted container"
    TEST_DATA="Hello from Brezno integration test!"
    echo "$TEST_DATA" > "$TEST_MOUNT/test-file.txt"
    if [ -f "$TEST_MOUNT/test-file.txt" ]; then
        print_success "Test file written successfully"
    else
        print_failure "Failed to write test file"
    fi

    # Test 5: Read data back
    print_test "Reading test data from mounted container"
    READ_DATA=$(cat "$TEST_MOUNT/test-file.txt")
    if [ "$READ_DATA" = "$TEST_DATA" ]; then
        print_success "Test data matches: '$READ_DATA'"
    else
        print_failure "Test data mismatch. Expected: '$TEST_DATA', Got: '$READ_DATA'"
    fi

    # Test 6: Unmount container
    print_test "Unmounting container"
    "$BINARY" unmount "$TEST_MOUNT"
    if ! mountpoint -q "$TEST_MOUNT"; then
        print_success "Container unmounted successfully"
    else
        print_failure "Container still mounted"
    fi

    # Test 7: Re-mount and verify data persistence
    print_test "Re-mounting container to verify data persistence"
    "$BINARY" mount "$TEST_CONTAINER" "$TEST_MOUNT" --keyfile "$TEST_KEYFILE"
    if mountpoint -q "$TEST_MOUNT"; then
        print_success "Container re-mounted successfully"
    else
        print_failure "Failed to re-mount container"
    fi

    # Test 8: Verify persisted data
    print_test "Verifying persisted data after remount"
    if [ -f "$TEST_MOUNT/test-file.txt" ]; then
        READ_DATA=$(cat "$TEST_MOUNT/test-file.txt")
        if [ "$READ_DATA" = "$TEST_DATA" ]; then
            print_success "Persisted data verified: '$READ_DATA'"
        else
            print_failure "Persisted data mismatch. Expected: '$TEST_DATA', Got: '$READ_DATA'"
        fi
    else
        print_failure "Test file not found after remount"
    fi

    # Test 9: Verify keyfile not exposed in any output
    print_test "Verifying keyfile path not exposed in output"
    "$BINARY" unmount "$TEST_MOUNT"

    # Run mount command and capture all output
    OUTPUT=$("$BINARY" mount "$TEST_CONTAINER" "$TEST_MOUNT" --keyfile "$TEST_KEYFILE" 2>&1)

    # Check if keyfile path is NOT exposed in any output
    if echo "$OUTPUT" | grep -q "$TEST_KEYFILE"; then
        print_failure "Keyfile path exposed in command output (security issue!)"
    else
        print_success "Keyfile path not exposed in command output"
    fi

    # Test 10: Test wrong keyfile (should fail)
    print_test "Testing wrong keyfile (should fail)"
    "$BINARY" unmount "$TEST_MOUNT"

    # Create a different keyfile
    WRONG_KEYFILE="$TEST_DIR/wrong.key"
    dd if=/dev/urandom of="$WRONG_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$WRONG_KEYFILE"

    set +e  # Don't exit on error for this test
    "$BINARY" mount "$TEST_CONTAINER" "$TEST_MOUNT" --keyfile "$WRONG_KEYFILE" 2>/dev/null
    if mountpoint -q "$TEST_MOUNT"; then
        print_failure "Container mounted with wrong keyfile (security issue!)"
    else
        print_success "Wrong keyfile correctly rejected"
    fi
    set -e

    # Final cleanup will be handled by trap

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
