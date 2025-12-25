# Brezno Integration Tests

This directory contains the integration test suite for Brezno.

## Structure

```
test/
├── integration_test.sh       # Main test runner
└── tests/
    ├── test_basic.sh         # Basic operations tests (create, mount, unmount, security)
    └── test_resize.sh        # Resize functionality tests (can run independently)
```

## Running Tests

The integration tests require root privileges:

```bash
# Build the binary first
go build -o brezno ./cmd/brezno

# Run all tests (default)
sudo ./test/integration_test.sh
sudo ./test/integration_test.sh all

# Run only basic tests
sudo ./test/integration_test.sh basic

# Run only resize tests (creates its own container automatically)
sudo ./test/integration_test.sh resize
```

## Test Independence

Each test module can run independently:
- **Basic tests**: Always create a fresh container
- **Resize tests**: Automatically create a container if one doesn't exist (from basic tests)

This allows you to:
- Run specific test modules in isolation for faster iteration
- Debug specific functionality without running the full suite
- Develop and test new features independently

## Test Modules

### test_basic.sh (11 tests)
- Container creation with keyfile and permission validation
- Mount, write data, and read data
- Unmount, remount, and verify data persistence
- Security tests (keyfile not exposed, wrong keyfile rejected)

### test_resize.sh (10 tests)
- Resize validation (unmounted container, smaller size - both should fail)
- Mount container and get initial filesystem size
- Resize from 100MB to 200MB and verify size increase
- Verify original data persists after resize
- Write additional data to expanded space
- Unmount/remount and verify all data persists

## Adding New Tests

To add new test modules:

1. Create a new test file in `tests/` directory (e.g., `test_feature.sh`)
2. Implement a function `run_feature_tests()` containing your tests
3. Use the provided helper functions:
   - `print_test "Test description"` - Start a new test
   - `print_success "Success message"` - Mark test as passed
   - `print_failure "Error message"` - Mark test as failed and exit
4. Source and call your test function in `integration_test.sh`:
   ```bash
   echo -e "\n${YELLOW}Running feature tests...${NC}"
   source "$SCRIPT_DIR/tests/test_feature.sh"
   run_feature_tests
   ```

## Available Variables

Test modules have access to these shared variables:

- `$BINARY` - Path to brezno binary
- `$TEST_DIR` - Temporary test directory
- `$TEST_CONTAINER` - Path to test container file
- `$TEST_MOUNT` - Path to test mount point
- `$TEST_KEYFILE` - Path to test keyfile
- `$TEST_DATA` - Shared test data string

## Test Counters

- `$TESTS_RUN` - Total number of tests executed
- `$TESTS_PASSED` - Number of tests passed
- `$TESTS_FAILED` - Number of tests failed

These are automatically updated by the helper functions.
