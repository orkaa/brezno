#!/bin/bash
# Restore operations test suite
# Tests: header restore, custom backup path, restore validation, safety checks

run_restore_tests() {
    # Create a fresh container for restore tests
    RESTORE_CONTAINER="$TEST_DIR/restore-test.img"
    RESTORE_KEYFILE="$TEST_DIR/restore-test.key"
    RESTORE_MOUNT="$TEST_DIR/restore-mount"

    print_test "Creating keyfile for restore tests"
    dd if=/dev/urandom of="$RESTORE_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$RESTORE_KEYFILE"
    if [ -f "$RESTORE_KEYFILE" ]; then
        print_success "Keyfile created successfully"
    else
        print_failure "Keyfile not created"
    fi

    print_test "Creating container for restore tests (50MB)"
    "$BINARY" create "$RESTORE_CONTAINER" --size 50M --keyfile "$RESTORE_KEYFILE"
    if [ -f "$RESTORE_CONTAINER" ]; then
        print_success "Container created successfully"
    else
        print_failure "Container file not created"
    fi

    print_test "Creating header backup for restore tests"
    "$BINARY" backup "$RESTORE_CONTAINER"
    EXPECTED_BACKUP="$RESTORE_CONTAINER.header.bak"
    if [ -f "$EXPECTED_BACKUP" ]; then
        print_success "Backup created at: $EXPECTED_BACKUP"
    else
        print_failure "Backup not created"
    fi

    print_test "Basic restore - default backup path with --yes"
    "$BINARY" restore "$RESTORE_CONTAINER" --yes
    RESULT=$?
    if [ $RESULT -eq 0 ]; then
        print_success "Restore completed successfully"
    else
        print_failure "Restore failed with exit code: $RESULT"
    fi

    print_test "Verifying container is still valid LUKS after restore"
    if cryptsetup isLuks "$RESTORE_CONTAINER" 2>/dev/null; then
        print_success "Container is valid LUKS after restore"
    else
        print_failure "Container is not valid LUKS after restore"
    fi

    print_test "Verifying container can be mounted after restore"
    mkdir -p "$RESTORE_MOUNT"
    "$BINARY" mount "$RESTORE_CONTAINER" "$RESTORE_MOUNT" --keyfile "$RESTORE_KEYFILE"
    if mountpoint -q "$RESTORE_MOUNT" 2>/dev/null; then
        print_success "Container can be mounted after restore"
        # Unmount for further tests
        "$BINARY" unmount "$RESTORE_CONTAINER"
    else
        print_failure "Container cannot be mounted after restore"
    fi

    print_test "Custom backup path restore"
    CUSTOM_BACKUP="$TEST_DIR/custom-restore-backup.header"
    "$BINARY" backup "$RESTORE_CONTAINER" --output "$CUSTOM_BACKUP"
    "$BINARY" restore "$RESTORE_CONTAINER" --backup "$CUSTOM_BACKUP" --yes
    RESULT=$?
    if [ $RESULT -eq 0 ]; then
        print_success "Restore from custom backup path successful"
    else
        print_failure "Restore from custom backup path failed"
    fi

    print_test "Mounted container rejection - restore should fail if mounted"
    "$BINARY" mount "$RESTORE_CONTAINER" "$RESTORE_MOUNT" --keyfile "$RESTORE_KEYFILE"

    set +e  # Don't exit on error for this test
    "$BINARY" restore "$RESTORE_CONTAINER" --yes 2>/dev/null
    RESULT=$?
    set -e

    # Unmount the container
    "$BINARY" unmount "$RESTORE_CONTAINER"

    if [ $RESULT -ne 0 ]; then
        print_success "Restore correctly rejected mounted container"
    else
        print_failure "Restore should have failed for mounted container"
    fi

    print_test "Invalid backup rejection - restore should fail with non-LUKS file"
    INVALID_BACKUP="$TEST_DIR/invalid-backup.dat"
    dd if=/dev/zero of="$INVALID_BACKUP" bs=1K count=1 2>/dev/null

    set +e  # Don't exit on error for this test
    "$BINARY" restore "$RESTORE_CONTAINER" --backup "$INVALID_BACKUP" --yes 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -ne 0 ]; then
        print_success "Restore correctly rejected invalid backup file"
    else
        print_failure "Restore should have failed for invalid backup file"
    fi

    print_test "Non-existent backup - restore should fail gracefully"
    set +e  # Don't exit on error for this test
    "$BINARY" restore "$RESTORE_CONTAINER" --backup "$TEST_DIR/does-not-exist.bak" --yes 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -ne 0 ]; then
        print_success "Restore correctly failed for non-existent backup"
    else
        print_failure "Restore should have failed for non-existent backup"
    fi

    print_test "Non-existent container - restore should fail gracefully"
    set +e  # Don't exit on error for this test
    "$BINARY" restore "$TEST_DIR/does-not-exist.img" --yes 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -ne 0 ]; then
        print_success "Restore correctly failed for non-existent container"
    else
        print_failure "Restore should have failed for non-existent container"
    fi

    print_test "Skip confirmation with --yes flag (verify flag works)"
    # Create a new backup to restore
    "$BINARY" backup "$RESTORE_CONTAINER" --output "$TEST_DIR/yes-test.bak" --yes

    # Restore with --yes (should succeed without prompting)
    "$BINARY" restore "$RESTORE_CONTAINER" --backup "$TEST_DIR/yes-test.bak" --yes
    RESULT=$?
    if [ $RESULT -eq 0 ]; then
        print_success "--yes flag allows skipping confirmation"
    else
        print_failure "--yes flag restore failed"
    fi

    # Cleanup restore test files
    rm -rf "$RESTORE_CONTAINER" "$RESTORE_KEYFILE" "$EXPECTED_BACKUP" "$CUSTOM_BACKUP" "$INVALID_BACKUP" "$RESTORE_MOUNT" "$TEST_DIR/yes-test.bak"
}
