#!/bin/bash
# Info operations test suite
# Tests: info on unmounted container, mounted container, JSON output, error cases

run_info_tests() {
    # Create a fresh container for info tests
    INFO_CONTAINER="$TEST_DIR/info-test.img"
    INFO_KEYFILE="$TEST_DIR/info-test.key"
    INFO_MOUNT="$TEST_DIR/info-mount"

    print_test "Creating keyfile for info tests"
    dd if=/dev/urandom of="$INFO_KEYFILE" bs=512 count=1 2>/dev/null
    chmod 600 "$INFO_KEYFILE"
    if [ -f "$INFO_KEYFILE" ]; then
        print_success "Keyfile created successfully"
    else
        print_failure "Keyfile not created"
    fi

    print_test "Creating container for info tests (100MB)"
    "$BINARY" create "$INFO_CONTAINER" --size 100M --keyfile "$INFO_KEYFILE"
    if [ -f "$INFO_CONTAINER" ]; then
        print_success "Container created successfully"
    else
        print_failure "Container file not created"
    fi

    print_test "Info on unmounted container (should show file and LUKS header info)"
    set +e
    INFO_OUTPUT=$("$BINARY" info "$INFO_CONTAINER" --no-color 2>&1)
    RESULT=$?
    set -e

    if [ $RESULT -eq 0 ]; then
        # Check for expected content in output
        if echo "$INFO_OUTPUT" | grep -q "Container:" && \
           echo "$INFO_OUTPUT" | grep -q "File Size:" && \
           echo "$INFO_OUTPUT" | grep -q "LUKS Header:" && \
           echo "$INFO_OUTPUT" | grep -q "Status: Not mounted"; then
            print_success "Info on unmounted container shows expected information"
        else
            print_failure "Info on unmounted container missing expected fields"
        fi
    else
        print_failure "Info command failed on unmounted container"
    fi

    print_test "Info shows LUKS version correctly"
    set +e
    INFO_OUTPUT=$("$BINARY" info "$INFO_CONTAINER" --no-color 2>&1)
    set -e

    if echo "$INFO_OUTPUT" | grep -q "Version: LUKS2"; then
        print_success "LUKS version displayed correctly"
    else
        print_failure "LUKS version not displayed or incorrect"
    fi

    # Create mount point for mounted container tests
    mkdir -p "$INFO_MOUNT"

    print_test "Mounting container for mounted status tests"
    "$BINARY" mount "$INFO_CONTAINER" "$INFO_MOUNT" --keyfile "$INFO_KEYFILE" 2>/dev/null
    if mountpoint -q "$INFO_MOUNT"; then
        print_success "Container mounted successfully"
    else
        print_failure "Container mount failed"
    fi

    print_test "Info on mounted container (should show mount status and disk usage)"
    set +e
    INFO_OUTPUT=$("$BINARY" info "$INFO_CONTAINER" --no-color 2>&1)
    RESULT=$?
    set -e

    if [ $RESULT -eq 0 ]; then
        # Check for mount-specific information
        if echo "$INFO_OUTPUT" | grep -q "Status: Mounted" && \
           echo "$INFO_OUTPUT" | grep -q "Mapper:" && \
           echo "$INFO_OUTPUT" | grep -q "Loop Device:" && \
           echo "$INFO_OUTPUT" | grep -q "Mount Point:" && \
           echo "$INFO_OUTPUT" | grep -q "Disk Usage:"; then
            print_success "Info on mounted container shows mount status and disk usage"
        else
            print_failure "Info on mounted container missing mount details"
        fi
    else
        print_failure "Info command failed on mounted container"
    fi

    print_test "Info shows filesystem type for mounted container"
    set +e
    INFO_OUTPUT=$("$BINARY" info "$INFO_CONTAINER" --no-color 2>&1)
    set -e

    if echo "$INFO_OUTPUT" | grep -q "Filesystem: ext4"; then
        print_success "Filesystem type displayed correctly"
    else
        print_failure "Filesystem type not displayed or incorrect"
    fi

    print_test "JSON output format validation"
    set +e
    JSON_OUTPUT=$("$BINARY" info "$INFO_CONTAINER" --json 2>&1)
    RESULT=$?
    set -e

    if [ $RESULT -eq 0 ]; then
        # Check that output contains expected JSON fields
        if echo "$JSON_OUTPUT" | grep -q '"container_path"' && \
           echo "$JSON_OUTPUT" | grep -q '"file_size_bytes"' && \
           echo "$JSON_OUTPUT" | grep -q '"luks_version"' && \
           echo "$JSON_OUTPUT" | grep -q '"is_active"' && \
           echo "$JSON_OUTPUT" | grep -q '"mapper_name"'; then
            print_success "JSON output contains expected fields"
        else
            print_failure "JSON output missing expected fields"
        fi
    else
        print_failure "JSON output command failed"
    fi

    print_test "JSON output shows is_active as true for mounted container"
    set +e
    JSON_OUTPUT=$("$BINARY" info "$INFO_CONTAINER" --json 2>&1)
    set -e

    if echo "$JSON_OUTPUT" | grep -q '"is_active": true'; then
        print_success "JSON correctly shows container as active"
    else
        print_failure "JSON should show is_active: true for mounted container"
    fi

    # Unmount for non-mounted tests
    print_test "Unmounting container for non-mounted tests"
    "$BINARY" unmount "$INFO_MOUNT" 2>/dev/null
    if ! mountpoint -q "$INFO_MOUNT"; then
        print_success "Container unmounted successfully"
    else
        print_failure "Container unmount failed"
    fi

    print_test "JSON output shows is_active as false for unmounted container"
    set +e
    JSON_OUTPUT=$("$BINARY" info "$INFO_CONTAINER" --json 2>&1)
    set -e

    if echo "$JSON_OUTPUT" | grep -q '"is_active": false'; then
        print_success "JSON correctly shows container as inactive"
    else
        print_failure "JSON should show is_active: false for unmounted container"
    fi

    print_test "Info on non-LUKS file should show file info but indicate not LUKS"
    NON_LUKS_FILE="$TEST_DIR/not-luks-info.img"
    dd if=/dev/zero of="$NON_LUKS_FILE" bs=1M count=1 2>/dev/null

    set +e
    INFO_OUTPUT=$("$BINARY" info "$NON_LUKS_FILE" --no-color 2>&1)
    RESULT=$?
    set -e

    # Should show file info but indicate it's not LUKS
    if echo "$INFO_OUTPUT" | grep -q "File Size:" && \
       echo "$INFO_OUTPUT" | grep -q "not a valid LUKS container"; then
        print_success "Info correctly shows file info and warns about non-LUKS file"
    else
        print_failure "Info should show file info and warn for non-LUKS file"
    fi

    print_test "Info on non-existent file should fail"
    set +e
    "$BINARY" info "$TEST_DIR/does-not-exist.img" 2>/dev/null
    RESULT=$?
    set -e

    if [ $RESULT -ne 0 ]; then
        print_success "Info correctly rejected non-existent file"
    else
        print_failure "Info should have failed for non-existent file"
    fi

    print_test "Info shows file size matches actual container size"
    ACTUAL_SIZE=$(stat -c %s "$INFO_CONTAINER" 2>/dev/null || stat -f %z "$INFO_CONTAINER" 2>/dev/null)
    set +e
    JSON_OUTPUT=$("$BINARY" info "$INFO_CONTAINER" --json 2>&1)
    set -e

    # Extract file_size_bytes from JSON (simple grep approach)
    if echo "$JSON_OUTPUT" | grep -q "\"file_size_bytes\": $ACTUAL_SIZE"; then
        print_success "File size in info matches actual file size"
    else
        print_failure "File size mismatch between info and actual file"
    fi

    # Cleanup info test files
    rm -f "$INFO_CONTAINER" "$INFO_KEYFILE" "$NON_LUKS_FILE"
    rm -rf "$INFO_MOUNT"
}
