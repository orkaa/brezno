#!/bin/bash
# Resize operations test suite
# Tests: resize validation, resize operation, data integrity after resize

# Setup function - creates a container if needed for isolated testing
setup_resize_tests() {
    # Check if container already exists (from basic tests)
    if [ ! -f "$TEST_CONTAINER" ]; then
        print_test "Setting up test container for resize tests"

        # Create keyfile if it doesn't exist
        if [ ! -f "$TEST_KEYFILE" ]; then
            dd if=/dev/urandom of="$TEST_KEYFILE" bs=512 count=1 2>/dev/null
            chmod 600 "$TEST_KEYFILE"
        fi

        # Create container
        "$BINARY" create "$TEST_CONTAINER" --size 100M --keyfile "$TEST_KEYFILE" 2>&1 | grep -v "^\[" || true

        # Mount it and write test data
        "$BINARY" mount "$TEST_CONTAINER" "$TEST_MOUNT" --keyfile "$TEST_KEYFILE" 2>&1 | grep -v "^\[" || true

        # Write test data
        TEST_DATA="Hello from Brezno integration test!"
        echo "$TEST_DATA" > "$TEST_MOUNT/test-file.txt"

        # Unmount it
        "$BINARY" unmount "$TEST_MOUNT" 2>&1 | grep -v "^\[" || true

        print_success "Test container created and ready"
    fi

    # Ensure TEST_DATA is set (needed for verification tests)
    if [ -z "$TEST_DATA" ]; then
        TEST_DATA="Hello from Brezno integration test!"
    fi
}

run_resize_tests() {
    # Setup container if running in isolation
    setup_resize_tests

    print_test "Testing resize on unmounted container (should fail)"
    # Ensure container is unmounted
    if mountpoint -q "$TEST_MOUNT" 2>/dev/null; then
        "$BINARY" unmount "$TEST_MOUNT"
    fi

    set +e  # Don't exit on error for this test
    "$BINARY" resize "$TEST_CONTAINER" --size 200M --keyfile "$TEST_KEYFILE" --yes 2>/dev/null
    RESIZE_EXIT_CODE=$?
    if [ $RESIZE_EXIT_CODE -ne 0 ]; then
        print_success "Resize correctly rejected for unmounted container"
    else
        print_failure "Resize should fail for unmounted container"
    fi
    set -e

    print_test "Mounting container for resize tests"
    "$BINARY" mount "$TEST_CONTAINER" "$TEST_MOUNT" --keyfile "$TEST_KEYFILE"
    if mountpoint -q "$TEST_MOUNT"; then
        print_success "Container mounted for resize tests"
    else
        print_failure "Failed to mount container"
    fi

    print_test "Getting filesystem size before resize"
    SIZE_BEFORE=$(df --block-size=1 "$TEST_MOUNT" | tail -1 | awk '{print $2}')
    if [ -n "$SIZE_BEFORE" ]; then
        print_success "Filesystem size before resize: $SIZE_BEFORE bytes"
    else
        print_failure "Failed to get filesystem size"
    fi

    print_test "Testing resize with smaller size (should fail)"
    set +e  # Don't exit on error for this test
    "$BINARY" resize "$TEST_CONTAINER" --size 50M --keyfile "$TEST_KEYFILE" --yes 2>/dev/null
    RESIZE_EXIT_CODE=$?
    if [ $RESIZE_EXIT_CODE -ne 0 ]; then
        print_success "Resize correctly rejected for smaller size"
    else
        print_failure "Resize should fail for smaller size"
    fi
    set -e

    print_test "Resizing container from 100MB to 200MB"
    "$BINARY" resize "$TEST_CONTAINER" --size 200M --keyfile "$TEST_KEYFILE" --yes

    # Verify container file size increased
    FILE_SIZE=$(stat -c "%s" "$TEST_CONTAINER")
    EXPECTED_SIZE=209715200  # 200MB in bytes
    if [ "$FILE_SIZE" -eq "$EXPECTED_SIZE" ]; then
        print_success "Container file resized to 200MB ($FILE_SIZE bytes)"
    else
        print_failure "Container file size incorrect. Expected: $EXPECTED_SIZE, Got: $FILE_SIZE"
    fi

    print_test "Verifying filesystem size increased after resize"
    SIZE_AFTER=$(df --block-size=1 "$TEST_MOUNT" | tail -1 | awk '{print $2}')
    if [ -n "$SIZE_AFTER" ] && [ "$SIZE_AFTER" -gt "$SIZE_BEFORE" ]; then
        print_success "Filesystem size increased: $SIZE_BEFORE → $SIZE_AFTER bytes"
    else
        print_failure "Filesystem size did not increase. Before: $SIZE_BEFORE, After: $SIZE_AFTER"
    fi

    print_test "Verifying data persists after resize"
    if [ -f "$TEST_MOUNT/test-file.txt" ]; then
        READ_DATA=$(cat "$TEST_MOUNT/test-file.txt")
        if [ "$READ_DATA" = "$TEST_DATA" ]; then
            print_success "Data verified after resize: '$READ_DATA'"
        else
            print_failure "Data mismatch after resize. Expected: '$TEST_DATA', Got: '$READ_DATA'"
        fi
    else
        print_failure "Test file not found after resize"
    fi

    print_test "Writing additional data to verify expanded space"
    dd if=/dev/zero of="$TEST_MOUNT/large-file.bin" bs=1M count=50 2>/dev/null
    if [ -f "$TEST_MOUNT/large-file.bin" ]; then
        LARGE_FILE_SIZE=$(stat -c "%s" "$TEST_MOUNT/large-file.bin")
        EXPECTED_LARGE_SIZE=52428800  # 50MB in bytes
        if [ "$LARGE_FILE_SIZE" -eq "$EXPECTED_LARGE_SIZE" ]; then
            print_success "Successfully wrote 50MB to expanded space"
        else
            print_failure "Large file size incorrect. Expected: $EXPECTED_LARGE_SIZE, Got: $LARGE_FILE_SIZE"
        fi
    else
        print_failure "Failed to write large file to expanded space"
    fi

    print_test "Unmounting and remounting to verify data persistence after resize"
    "$BINARY" unmount "$TEST_MOUNT"
    if ! mountpoint -q "$TEST_MOUNT"; then
        print_success "Container unmounted"
    else
        print_failure "Failed to unmount container"
    fi

    "$BINARY" mount "$TEST_CONTAINER" "$TEST_MOUNT" --keyfile "$TEST_KEYFILE"
    if mountpoint -q "$TEST_MOUNT"; then
        print_success "Container re-mounted after resize"
    else
        print_failure "Failed to re-mount container"
    fi

    print_test "Verifying all data persists after resize and remount"
    VERIFIED=0

    # Check original test file
    if [ -f "$TEST_MOUNT/test-file.txt" ]; then
        READ_DATA=$(cat "$TEST_MOUNT/test-file.txt")
        if [ "$READ_DATA" = "$TEST_DATA" ]; then
            VERIFIED=$((VERIFIED + 1))
        fi
    fi

    # Check large file
    if [ -f "$TEST_MOUNT/large-file.bin" ]; then
        LARGE_FILE_SIZE=$(stat -c "%s" "$TEST_MOUNT/large-file.bin")
        if [ "$LARGE_FILE_SIZE" -eq 52428800 ]; then
            VERIFIED=$((VERIFIED + 1))
        fi
    fi

    if [ $VERIFIED -eq 2 ]; then
        print_success "All data verified after resize and remount"
    else
        print_failure "Data verification failed. Verified: $VERIFIED/2 files"
    fi

    # Final unmount before cleanup
    "$BINARY" unmount "$TEST_MOUNT"
}
