#!/bin/bash
# Basic operations test suite
# Tests: create, mount, unmount, data persistence, security

run_basic_tests() {
    print_test "Creating test keyfile"
    dd if=/dev/urandom of="$TEST_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$TEST_KEYFILE"
    if [ -f "$TEST_KEYFILE" ]; then
        print_success "Keyfile created successfully"
    else
        print_failure "Keyfile not created"
    fi

    print_test "Creating encrypted container (100MB) with keyfile"
    "$BINARY" create "$TEST_CONTAINER" --size 100M --keyfile "$TEST_KEYFILE"
    if [ -f "$TEST_CONTAINER" ]; then
        print_success "Container created successfully"
    else
        print_failure "Container file not created"
    fi

    print_test "Verifying secure file permissions (0600)"
    PERMS=$(stat -c "%a" "$TEST_CONTAINER")
    if [ "$PERMS" = "600" ]; then
        print_success "Container has secure permissions: $PERMS (owner-only)"
    else
        print_failure "Container has insecure permissions: $PERMS (expected 600)"
    fi

    print_test "Mounting container with keyfile"
    "$BINARY" mount "$TEST_CONTAINER" "$TEST_MOUNT" --keyfile "$TEST_KEYFILE"
    if mountpoint -q "$TEST_MOUNT"; then
        print_success "Container mounted successfully"
    else
        print_failure "Container not mounted"
    fi

    print_test "Writing test data to mounted container"
    TEST_DATA="Hello from Brezno integration test!"
    echo "$TEST_DATA" > "$TEST_MOUNT/test-file.txt"
    if [ -f "$TEST_MOUNT/test-file.txt" ]; then
        print_success "Test file written successfully"
    else
        print_failure "Failed to write test file"
    fi

    print_test "Reading test data from mounted container"
    READ_DATA=$(cat "$TEST_MOUNT/test-file.txt")
    if [ "$READ_DATA" = "$TEST_DATA" ]; then
        print_success "Test data matches: '$READ_DATA'"
    else
        print_failure "Test data mismatch. Expected: '$TEST_DATA', Got: '$READ_DATA'"
    fi

    print_test "Unmounting container"
    "$BINARY" unmount "$TEST_MOUNT"
    if ! mountpoint -q "$TEST_MOUNT"; then
        print_success "Container unmounted successfully"
    else
        print_failure "Container still mounted"
    fi

    print_test "Re-mounting container to verify data persistence"
    "$BINARY" mount "$TEST_CONTAINER" "$TEST_MOUNT" --keyfile "$TEST_KEYFILE"
    if mountpoint -q "$TEST_MOUNT"; then
        print_success "Container re-mounted successfully"
    else
        print_failure "Failed to re-mount container"
    fi

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
}
