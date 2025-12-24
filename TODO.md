# Security Fixes TODO

## Status Legend
- [ ] To Do
- [x] Completed

## High Priority Vulnerabilities

### [x] Fix #1: Password Memory Exposure
**Severity:** HIGH
**Files:** `internal/system/secure.go` (new), `internal/container/luks.go`, `internal/ui/prompt.go`, `internal/cli/create.go`, `internal/cli/mount.go`
**Description:** Replace string-based password storage with SecureBytes type that zeros memory after use.
**Testing:** Verify passwords zeroed after use, test container creation/mounting workflows.

### [x] Fix #2: Debug Command Exposure
**Severity:** HIGH
**Files:** `internal/system/executor.go`
**Description:** Sanitize debug output to redact sensitive arguments (keyfiles, passwords).
**Testing:** Run with `--debug` flag, verify sensitive args show as `[REDACTED]`.
**Note:** Debug flag handling has a pre-existing bug (fixed separately), but sanitization code is in place.

## Medium Priority Vulnerabilities

### [ ] Fix #3: Insecure File Creation
**Severity:** MEDIUM-HIGH
**Files:** `internal/cli/create.go`
**Description:** Use `os.OpenFile()` with `0600` permissions and `O_EXCL` flag for atomic creation.
**Testing:** Verify new containers have `rw-------` permissions, test atomicity with concurrent creates.

### [ ] Fix #4: Keyfile Path Injection
**Severity:** MEDIUM
**Files:** `internal/system/pathutil.go` (new), `internal/cli/create.go`, `internal/cli/mount.go`
**Description:** Validate and resolve keyfile paths, prevent symlink attacks.
**Testing:** Test with symlinks, directories, non-existent files, insecure permissions.

### [ ] Fix #5: TOCTOU Race Condition
**Severity:** MEDIUM
**Files:** `internal/cli/create.go`
**Description:** Remove redundant file existence check (already fixed by Fix #3's O_EXCL).
**Dependencies:** Must complete Fix #3 first.
**Testing:** Test concurrent file creation attempts.

## Notes
- All fixes maintain backward compatibility with existing containers
- Each fix should be a separate commit for clean history
- Test each fix independently before moving to the next
- Existing containers will continue to work without modification
