#!/bin/bash
# Backup operations test suite
# Tests: header backup, custom output path, backup verification, overwrite protection

run_backup_tests() {
    # Create a fresh container for backup tests
    BACKUP_CONTAINER="$TEST_DIR/backup-test.img"
    BACKUP_KEYFILE="$TEST_DIR/backup-test.key"

    print_test "Creating keyfile for backup tests"
    dd if=/dev/urandom of="$BACKUP_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$BACKUP_KEYFILE"
    if [ -f "$BACKUP_KEYFILE" ]; then
        print_success "Keyfile created successfully"
    else
        print_failure "Keyfile not created"
    fi

    print_test "Creating container for backup tests (50MB)"
    "$BINARY" create "$BACKUP_CONTAINER" --size 50M --keyfile "$BACKUP_KEYFILE"
    if [ -f "$BACKUP_CONTAINER" ]; then
        print_success "Container created successfully"
    else
        print_failure "Container file not created"
    fi

    print_test "Basic backup - default output path"
    "$BINARY" backup "$BACKUP_CONTAINER"
    EXPECTED_BACKUP="$BACKUP_CONTAINER.header.bak"
    if [ -f "$EXPECTED_BACKUP" ]; then
        print_success "Backup created at default path: $EXPECTED_BACKUP"
    else
        print_failure "Backup not created at expected path: $EXPECTED_BACKUP"
    fi

    print_test "Verifying backup file permissions (0600)"
    PERMS=$(stat -c "%a" "$EXPECTED_BACKUP")
    if [ "$PERMS" = "600" ]; then
        print_success "Backup has secure permissions: $PERMS"
    else
        print_failure "Backup has insecure permissions: $PERMS (expected 600)"
    fi

    print_test "Verifying backup is valid LUKS header"
    if cryptsetup isLuks "$EXPECTED_BACKUP" 2>/dev/null; then
        print_success "Backup is a valid LUKS header"
    else
        print_failure "Backup is not a valid LUKS header"
    fi

    print_test "Custom output path backup"
    CUSTOM_BACKUP="$TEST_DIR/custom-backup.header"
    "$BINARY" backup "$BACKUP_CONTAINER" --output "$CUSTOM_BACKUP"
    if [ -f "$CUSTOM_BACKUP" ]; then
        print_success "Backup created at custom path: $CUSTOM_BACKUP"
    else
        print_failure "Backup not created at custom path: $CUSTOM_BACKUP"
    fi

    print_test "Custom backup is valid LUKS header"
    if cryptsetup isLuks "$CUSTOM_BACKUP" 2>/dev/null; then
        print_success "Custom backup is a valid LUKS header"
    else
        print_failure "Custom backup is not a valid LUKS header"
    fi

    print_test "Overwrite protection - should fail without --yes"
    # Create a file at the target path
    OVERWRITE_TARGET="$TEST_DIR/overwrite-test.bak"
    echo "existing content" > "$OVERWRITE_TARGET"

    set +e  # Don't exit on error for this test
    # Run without --yes, pipe 'n' to stdin to decline overwrite
    echo "n" | "$BINARY" backup "$BACKUP_CONTAINER" --output "$OVERWRITE_TARGET" 2>/dev/null
    RESULT=$?
    set -e

    # Check that file was not overwritten
    CONTENT=$(cat "$OVERWRITE_TARGET")
    if [ "$CONTENT" = "existing content" ]; then
        print_success "Overwrite protection works - file not overwritten"
    else
        print_failure "Overwrite protection failed - file was overwritten"
    fi

    print_test "Skip confirmation with --yes flag"
    "$BINARY" backup "$BACKUP_CONTAINER" --output "$OVERWRITE_TARGET" --yes
    if cryptsetup isLuks "$OVERWRITE_TARGET" 2>/dev/null; then
        print_success "--yes flag allows overwriting existing file"
    else
        print_failure "Backup with --yes flag failed"
    fi

    print_test "Backup non-LUKS file should fail"
    NON_LUKS_FILE="$TEST_DIR/not-luks.img"
    dd if=/dev/zero of="$NON_LUKS_FILE" bs=1M count=1 2>/dev/null

    set +e  # Don't exit on error for this test
    "$BINARY" backup "$NON_LUKS_FILE" 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -ne 0 ]; then
        print_success "Backup correctly rejected non-LUKS file"
    else
        print_failure "Backup should have failed for non-LUKS file"
    fi

    print_test "Backup non-existent file should fail"
    set +e  # Don't exit on error for this test
    "$BINARY" backup "$TEST_DIR/does-not-exist.img" 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -ne 0 ]; then
        print_success "Backup correctly rejected non-existent file"
    else
        print_failure "Backup should have failed for non-existent file"
    fi

    # Cleanup backup test files
    rm -f "$BACKUP_CONTAINER" "$BACKUP_KEYFILE" "$EXPECTED_BACKUP" "$CUSTOM_BACKUP" "$OVERWRITE_TARGET" "$NON_LUKS_FILE"
}
