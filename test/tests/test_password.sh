#!/bin/bash
# Password change operations test suite
# Tests: password→password, password→keyfile, keyfile→password, keyfile→keyfile,
#        error handling, security validations

# Setup function - creates containers for password testing
setup_password_tests() {
    # Ensure TEST_DATA is set
    if [ -z "$TEST_DATA" ]; then
        TEST_DATA="Password test data"
    fi

    # Define test containers and credentials
    PASS_CONTAINER="$TEST_DIR/pass-container.img"
    PASS_INITIAL_KEYFILE="$TEST_DIR/pass-initial.key"
    PASS_PASSWORD="TestPass123!"
    PASS_NEW_PASSWORD="NewPass456!"

    KEY_CONTAINER="$TEST_DIR/key-container.img"
    KEY_KEYFILE="$TEST_DIR/key1.key"
    KEY_NEW_KEYFILE="$TEST_DIR/key2.key"

    # Create all keyfiles
    dd if=/dev/urandom of="$PASS_INITIAL_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$PASS_INITIAL_KEYFILE"
    dd if=/dev/urandom of="$KEY_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$KEY_KEYFILE"
    dd if=/dev/urandom of="$KEY_NEW_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$KEY_NEW_KEYFILE"

    # Create both containers with keyfiles initially
    # (We'll convert PASS_CONTAINER to password-based auth after)
    "$BINARY" create "$PASS_CONTAINER" --size 50M --keyfile "$PASS_INITIAL_KEYFILE" 2>&1 | grep -v "^\[" || return 1
    "$BINARY" create "$KEY_CONTAINER" --size 50M --keyfile "$KEY_KEYFILE" 2>&1 | grep -v "^\[" || return 1

    # Add test data to both containers
    "$BINARY" mount "$PASS_CONTAINER" "$TEST_MOUNT" --keyfile "$PASS_INITIAL_KEYFILE" 2>&1 | grep -v "^\[" || true
    echo "$TEST_DATA" > "$TEST_MOUNT/test-file.txt"
    "$BINARY" unmount "$TEST_MOUNT" 2>&1 | grep -v "^\[" || true

    "$BINARY" mount "$KEY_CONTAINER" "$TEST_MOUNT" --keyfile "$KEY_KEYFILE" 2>&1 | grep -v "^\[" || true
    echo "$TEST_DATA" > "$TEST_MOUNT/test-file.txt"
    "$BINARY" unmount "$TEST_MOUNT" 2>&1 | grep -v "^\[" || true

    # Convert PASS_CONTAINER to password-based authentication for password→password tests
    printf "%s\n%s\n" "$PASS_PASSWORD" "$PASS_PASSWORD" | "$BINARY" password "$PASS_CONTAINER" --keyfile "$PASS_INITIAL_KEYFILE" --password-stdin 2>&1 | grep -v "^\[" || true
}

run_password_tests() {
    # Setup containers for password tests
    print_test "Setting up test containers for password tests"
    setup_password_tests
    print_success "Test containers created"

    # =================================================================
    # Test 1: Password → Password change
    # =================================================================
    print_test "Changing password from password to password"

    # Change password
    printf "%s\n%s\n%s\n" "$PASS_PASSWORD" "$PASS_NEW_PASSWORD" "$PASS_NEW_PASSWORD" | "$BINARY" password "$PASS_CONTAINER" --password-stdin 2>&1 | grep -v "^\[" || true

    # Verify old password no longer works
    print_test "Verifying old password is rejected"
    set +e
    echo "$PASS_PASSWORD" | "$BINARY" mount "$PASS_CONTAINER" "$TEST_MOUNT" --password-stdin 2>/dev/null
    if mountpoint -q "$TEST_MOUNT"; then
        print_failure "Old password still works (password change failed)"
    else
        print_success "Old password correctly rejected"
    fi
    set -e

    # Verify new password works and data is intact
    print_test "Verifying new password works and data persists"
    echo "$PASS_NEW_PASSWORD" | "$BINARY" mount "$PASS_CONTAINER" "$TEST_MOUNT" --password-stdin 2>&1 | grep -v "^\[" || true
    if mountpoint -q "$TEST_MOUNT"; then
        READ_DATA=$(cat "$TEST_MOUNT/test-file.txt")
        if [ "$READ_DATA" = "$TEST_DATA" ]; then
            print_success "New password works, data intact: '$READ_DATA'"
        else
            print_failure "Data mismatch. Expected: '$TEST_DATA', Got: '$READ_DATA'"
        fi
    else
        print_failure "Failed to mount with new password"
    fi
    "$BINARY" unmount "$TEST_MOUNT"

    # =================================================================
    # Test 2: Password → Keyfile change
    # =================================================================
    print_test "Changing authentication from password to keyfile"

    PASS_TO_KEY_KEYFILE="$TEST_DIR/pass-to-key.key"
    dd if=/dev/urandom of="$PASS_TO_KEY_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$PASS_TO_KEY_KEYFILE"

    # Change from password to keyfile
    echo "$PASS_NEW_PASSWORD" | "$BINARY" password "$PASS_CONTAINER" --new-keyfile "$PASS_TO_KEY_KEYFILE" --password-stdin 2>&1 | grep -v "^\[" || true

    # Verify password no longer works
    print_test "Verifying password is rejected after switch to keyfile"
    set +e
    echo "$PASS_NEW_PASSWORD" | "$BINARY" mount "$PASS_CONTAINER" "$TEST_MOUNT" --password-stdin 2>/dev/null
    if mountpoint -q "$TEST_MOUNT"; then
        print_failure "Password still works after switch to keyfile"
    else
        print_success "Password correctly rejected after switch"
    fi
    set -e

    # Verify keyfile works and data is intact
    print_test "Verifying keyfile works and data persists after switch"
    "$BINARY" mount "$PASS_CONTAINER" "$TEST_MOUNT" --keyfile "$PASS_TO_KEY_KEYFILE" 2>&1 | grep -v "^\[" || true
    if mountpoint -q "$TEST_MOUNT"; then
        READ_DATA=$(cat "$TEST_MOUNT/test-file.txt")
        if [ "$READ_DATA" = "$TEST_DATA" ]; then
            print_success "Keyfile works, data intact: '$READ_DATA'"
        else
            print_failure "Data mismatch. Expected: '$TEST_DATA', Got: '$READ_DATA'"
        fi
    else
        print_failure "Failed to mount with new keyfile"
    fi
    "$BINARY" unmount "$TEST_MOUNT"

    # =================================================================
    # Test 3: Keyfile → Keyfile change
    # =================================================================
    print_test "Changing keyfile from keyfile to keyfile"

    # Change from key1 to key2
    "$BINARY" password "$KEY_CONTAINER" --keyfile "$KEY_KEYFILE" --new-keyfile "$KEY_NEW_KEYFILE" 2>&1 | grep -v "^\[" || true

    # Verify old keyfile no longer works
    print_test "Verifying old keyfile is rejected"
    set +e
    "$BINARY" mount "$KEY_CONTAINER" "$TEST_MOUNT" --keyfile "$KEY_KEYFILE" 2>/dev/null
    if mountpoint -q "$TEST_MOUNT"; then
        print_failure "Old keyfile still works (keyfile change failed)"
    else
        print_success "Old keyfile correctly rejected"
    fi
    set -e

    # Verify new keyfile works and data is intact
    print_test "Verifying new keyfile works and data persists"
    "$BINARY" mount "$KEY_CONTAINER" "$TEST_MOUNT" --keyfile "$KEY_NEW_KEYFILE" 2>&1 | grep -v "^\[" || true
    if mountpoint -q "$TEST_MOUNT"; then
        READ_DATA=$(cat "$TEST_MOUNT/test-file.txt")
        if [ "$READ_DATA" = "$TEST_DATA" ]; then
            print_success "New keyfile works, data intact: '$READ_DATA'"
        else
            print_failure "Data mismatch. Expected: '$TEST_DATA', Got: '$READ_DATA'"
        fi
    else
        print_failure "Failed to mount with new keyfile"
    fi
    "$BINARY" unmount "$TEST_MOUNT"

    # =================================================================
    # Test 4: Keyfile → Password change
    # =================================================================
    print_test "Changing authentication from keyfile to password"

    KEY_TO_PASS_PASSWORD="KeyToPass789!"

    # Change from keyfile to password
    printf "%s\n%s\n" "$KEY_TO_PASS_PASSWORD" "$KEY_TO_PASS_PASSWORD" | "$BINARY" password "$KEY_CONTAINER" --keyfile "$KEY_NEW_KEYFILE" --password-stdin 2>&1 | grep -v "^\[" || true

    # Verify keyfile no longer works
    print_test "Verifying keyfile is rejected after switch to password"
    set +e
    "$BINARY" mount "$KEY_CONTAINER" "$TEST_MOUNT" --keyfile "$KEY_NEW_KEYFILE" 2>/dev/null
    if mountpoint -q "$TEST_MOUNT"; then
        print_failure "Keyfile still works after switch to password"
    else
        print_success "Keyfile correctly rejected after switch"
    fi
    set -e

    # Verify password works and data is intact
    print_test "Verifying password works and data persists after switch"
    echo "$KEY_TO_PASS_PASSWORD" | "$BINARY" mount "$KEY_CONTAINER" "$TEST_MOUNT" --password-stdin 2>&1 | grep -v "^\[" || true
    if mountpoint -q "$TEST_MOUNT"; then
        READ_DATA=$(cat "$TEST_MOUNT/test-file.txt")
        if [ "$READ_DATA" = "$TEST_DATA" ]; then
            print_success "Password works, data intact: '$READ_DATA'"
        else
            print_failure "Data mismatch. Expected: '$TEST_DATA', Got: '$READ_DATA'"
        fi
    else
        print_failure "Failed to mount with new password"
    fi
    "$BINARY" unmount "$TEST_MOUNT"

    # =================================================================
    # Test 5: Error handling - Wrong current password
    # =================================================================
    print_test "Testing wrong current password (should fail)"

    set +e
    printf "%s\n%s\n%s\n" "WrongPassword123!" "NewPassword456!" "NewPassword456!" | "$BINARY" password "$KEY_CONTAINER" --password-stdin 2>/dev/null
    WRONG_PASS_EXIT_CODE=$?

    if [ $WRONG_PASS_EXIT_CODE -ne 0 ]; then
        print_success "Wrong password correctly rejected"
    else
        print_failure "Password change should fail with wrong current password"
    fi
    set -e

    # =================================================================
    # Test 6: Error handling - Wrong current keyfile
    # =================================================================
    print_test "Testing wrong current keyfile (should fail)"

    WRONG_KEYFILE="$TEST_DIR/wrong.key"
    dd if=/dev/urandom of="$WRONG_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$WRONG_KEYFILE"

    set +e
    printf "%s\n%s\n" "NewPassword999!" "NewPassword999!" | "$BINARY" password "$KEY_CONTAINER" --keyfile "$WRONG_KEYFILE" --password-stdin 2>/dev/null
    WRONG_KEY_EXIT_CODE=$?

    if [ $WRONG_KEY_EXIT_CODE -ne 0 ]; then
        print_success "Wrong keyfile correctly rejected"
    else
        print_failure "Password change should fail with wrong keyfile"
    fi
    set -e

    # =================================================================
    # Test 7: Error handling - Mounted container (should fail)
    # =================================================================
    print_test "Testing password change on mounted container (should fail)"

    # Mount the container
    echo "$KEY_TO_PASS_PASSWORD" | "$BINARY" mount "$KEY_CONTAINER" "$TEST_MOUNT" --password-stdin 2>&1 | grep -v "^\[" || true

    set +e
    printf "%s\n%s\n%s\n" "$KEY_TO_PASS_PASSWORD" "ShouldNotWork123!" "ShouldNotWork123!" | "$BINARY" password "$KEY_CONTAINER" --password-stdin 2>/dev/null
    MOUNTED_EXIT_CODE=$?

    if [ $MOUNTED_EXIT_CODE -ne 0 ]; then
        print_success "Password change correctly rejected for mounted container"
    else
        print_failure "Password change should fail for mounted container"
    fi
    set -e

    # Unmount
    "$BINARY" unmount "$TEST_MOUNT"

    # =================================================================
    # Test 8: Error handling - Non-existent container
    # =================================================================
    print_test "Testing password change on non-existent container (should fail)"

    set +e
    printf "%s\n%s\n%s\n" "password" "newpassword" "newpassword" | "$BINARY" password "$TEST_DIR/nonexistent.img" --password-stdin 2>/dev/null
    NONEXIST_EXIT_CODE=$?

    if [ $NONEXIST_EXIT_CODE -ne 0 ]; then
        print_success "Non-existent container correctly rejected"
    else
        print_failure "Password change should fail for non-existent container"
    fi
    set -e

    # =================================================================
    # Test 9: Error handling - Password mismatch
    # =================================================================
    print_test "Testing password mismatch detection (should fail)"

    set +e
    printf "%s\n%s\n%s\n" "$KEY_TO_PASS_PASSWORD" "NewPassword111!" "DifferentPassword222!" | "$BINARY" password "$KEY_CONTAINER" --password-stdin 2>/dev/null
    MISMATCH_EXIT_CODE=$?

    if [ $MISMATCH_EXIT_CODE -ne 0 ]; then
        print_success "Password mismatch correctly detected"
    else
        print_failure "Password change should fail when passwords don't match"
    fi
    set -e

    # Verify the password wasn't changed by the failed attempt
    print_test "Verifying password unchanged after failed mismatch attempt"
    echo "$KEY_TO_PASS_PASSWORD" | "$BINARY" mount "$KEY_CONTAINER" "$TEST_MOUNT" --password-stdin 2>&1 | grep -v "^\[" || true
    if mountpoint -q "$TEST_MOUNT"; then
        print_success "Original password still works after failed change"
    else
        print_failure "Original password should still work"
    fi
    "$BINARY" unmount "$TEST_MOUNT"

    # =================================================================
    # Test 10: Security - Verify keyfile paths not exposed in output
    # =================================================================
    print_test "Verifying keyfile paths not exposed in command output"

    TEST_KEYFILE_SECURITY="$TEST_DIR/security-test.key"
    dd if=/dev/urandom of="$TEST_KEYFILE_SECURITY" bs=512 count=1 2>/dev/null
    chmod 600 "$TEST_KEYFILE_SECURITY"

    # Capture all output from password change
    OUTPUT=$(echo "$KEY_TO_PASS_PASSWORD" | "$BINARY" password "$KEY_CONTAINER" --new-keyfile "$TEST_KEYFILE_SECURITY" --password-stdin 2>&1)

    # Check if keyfile path is NOT exposed in any output
    if echo "$OUTPUT" | grep -q "$TEST_KEYFILE_SECURITY"; then
        print_failure "Keyfile path exposed in command output (security issue!)"
    else
        print_success "Keyfile path not exposed in command output"
    fi

    # Verify the change worked (now uses keyfile)
    "$BINARY" mount "$KEY_CONTAINER" "$TEST_MOUNT" --keyfile "$TEST_KEYFILE_SECURITY" 2>&1 | grep -v "^\[" || true
    "$BINARY" unmount "$TEST_MOUNT"

    # =================================================================
    # Test 11: Invalid keyfile path validation
    # =================================================================
    print_test "Testing invalid new keyfile path (should fail)"

    set +e
    echo "$KEY_TO_PASS_PASSWORD" | "$BINARY" password "$KEY_CONTAINER" --new-keyfile "/nonexistent/path/key.file" --password-stdin 2>/dev/null
    INVALID_PATH_EXIT_CODE=$?

    if [ $INVALID_PATH_EXIT_CODE -ne 0 ]; then
        print_success "Invalid keyfile path correctly rejected"
    else
        print_failure "Password change should fail with invalid keyfile path"
    fi
    set -e

    print_test "Cleaning up password test containers"
    rm -f "$PASS_CONTAINER" "$KEY_CONTAINER" "$PASS_INITIAL_KEYFILE" "$PASS_TO_KEY_KEYFILE" "$KEY_KEYFILE" "$KEY_NEW_KEYFILE" "$WRONG_KEYFILE" "$TEST_KEYFILE_SECURITY"
    print_success "Password test cleanup complete"
}
