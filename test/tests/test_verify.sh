#!/bin/bash
# Verify operations test suite
# Tests: header verification, full verification with keyfile/password, JSON output, error cases

run_verify_tests() {
    # Create a fresh container for verify tests
    VERIFY_CONTAINER="$TEST_DIR/verify-test.img"
    VERIFY_KEYFILE="$TEST_DIR/verify-test.key"
    VERIFY_PASSWORD="test-password-123"

    print_test "Creating keyfile for verify tests"
    dd if=/dev/urandom of="$VERIFY_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$VERIFY_KEYFILE"
    if [ -f "$VERIFY_KEYFILE" ]; then
        print_success "Keyfile created successfully"
    else
        print_failure "Keyfile not created"
    fi

    print_test "Creating container for verify tests (50MB)"
    "$BINARY" create "$VERIFY_CONTAINER" --size 50M --keyfile "$VERIFY_KEYFILE"
    if [ -f "$VERIFY_CONTAINER" ]; then
        print_success "Container created successfully"
    else
        print_failure "Container file not created"
    fi

    print_test "Basic header verification (should pass on valid LUKS container)"
    set +e
    "$BINARY" verify "$VERIFY_CONTAINER" 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -eq 0 ]; then
        print_success "Header verification passed for valid container"
    else
        print_failure "Header verification failed for valid container"
    fi

    print_test "Full verification with keyfile (should pass with correct keyfile)"
    set +e
    "$BINARY" verify "$VERIFY_CONTAINER" --full --keyfile "$VERIFY_KEYFILE" 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -eq 0 ]; then
        print_success "Full verification with keyfile passed"
    else
        print_failure "Full verification with keyfile failed"
    fi

    # Create a container with password for password tests
    PASSWORD_CONTAINER="$TEST_DIR/verify-password-test.img"
    print_test "Creating password-protected container for password tests"
    printf "%s\n%s\n" "$VERIFY_PASSWORD" "$VERIFY_PASSWORD" | "$BINARY" create "$PASSWORD_CONTAINER" --size 50M --password-stdin 2>/dev/null
    if [ -f "$PASSWORD_CONTAINER" ]; then
        print_success "Password-protected container created"
    else
        print_failure "Password-protected container not created"
    fi

    print_test "Full verification with password via stdin (should pass with correct password)"
    set +e
    echo "$VERIFY_PASSWORD" | "$BINARY" verify "$PASSWORD_CONTAINER" --full --password-stdin 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -eq 0 ]; then
        print_success "Full verification with password via stdin passed"
    else
        print_failure "Full verification with password via stdin failed"
    fi

    print_test "JSON output format validation"
    set +e
    JSON_OUTPUT=$("$BINARY" verify "$VERIFY_CONTAINER" --json 2>/dev/null)
    RESULT=$?
    set -e

    if [ $RESULT -eq 0 ]; then
        # Check that output contains expected JSON fields
        if echo "$JSON_OUTPUT" | grep -q '"container_path"' && \
           echo "$JSON_OUTPUT" | grep -q '"header_valid"' && \
           echo "$JSON_OUTPUT" | grep -q '"header_info"'; then
            print_success "JSON output contains expected fields"
        else
            print_failure "JSON output missing expected fields"
        fi
    else
        print_failure "JSON output command failed"
    fi

    print_test "Verify non-LUKS file should fail"
    NON_LUKS_FILE="$TEST_DIR/not-luks-verify.img"
    dd if=/dev/zero of="$NON_LUKS_FILE" bs=1M count=1 2>/dev/null

    set +e
    "$BINARY" verify "$NON_LUKS_FILE" 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -ne 0 ]; then
        print_success "Verify correctly rejected non-LUKS file"
    else
        print_failure "Verify should have failed for non-LUKS file"
    fi

    print_test "Verify non-existent file should fail"
    set +e
    "$BINARY" verify "$TEST_DIR/does-not-exist.img" 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -ne 0 ]; then
        print_success "Verify correctly rejected non-existent file"
    else
        print_failure "Verify should have failed for non-existent file"
    fi

    print_test "Full verification with wrong password should fail"
    set +e
    echo "wrong-password" | "$BINARY" verify "$PASSWORD_CONTAINER" --full --password-stdin 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -ne 0 ]; then
        print_success "Full verification correctly rejected wrong password"
    else
        print_failure "Full verification should have failed with wrong password"
    fi

    # Cleanup verify test files
    rm -f "$VERIFY_CONTAINER" "$VERIFY_KEYFILE" "$PASSWORD_CONTAINER" "$NON_LUKS_FILE"
}
