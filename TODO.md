# Code Cleanup TODO

## Status Legend
- [ ] To Do
- [x] Completed

## Phase 1: Bug Fixes (HIGH PRIORITY)

### [x] 1.1: Fix filepath.Abs() Error Handling
**Risk:** LOW
**Files:** `internal/container/discovery.go`
**Description:** Lines 65, 91, 124 ignore errors from `filepath.Abs()` using `_`. Return error immediately if unexpected.
**Solution:** Removed unnecessary Abs() call on line 65 (kernel provides absolute paths). Added error returns on lines 91, 124.
**Testing:** All 11 integration tests pass.

### [x] 1.2: Fix Cleanup Error Handling
**Risk:** LOW
**Files:** `internal/cli/create.go`
**Description:** Lines 194-196 silently ignore cleanup errors. Add logging for cleanup failures to match unmount.go pattern.
**Solution:** Added error checking and warning logs for Close() and Detach() operations. Matches unmount.go pattern.
**Testing:** All 11 integration tests pass.

### [x] 1.3: Fix Flag Shadowing
**Risk:** MEDIUM
**Files:** `internal/ui/logger.go`, `internal/cli/list.go`
**Description:** Local `verbose` flag shadows global `--verbose` flag. Use global flag instead of local one.
**Solution:** Made Logger fields public (idiomatic Go). Removed local verbose flag. List command now uses ctx.Logger.Verbose.
**Result:** Single source of truth, `brezno --verbose list` shows both detailed output AND info logs.
**Testing:** All 11 integration tests pass.

### [ ] 1.4: Fix filepath.Abs() Error Handling in unmount.go
**Risk:** LOW
**Files:** `internal/cli/unmount.go`
**Description:** Line 59 ignores filepath.Abs() error with `_`. Return error immediately if unexpected.
**Testing:** Integration tests should pass unchanged.

## Phase 2: Code Duplication (MEDIUM PRIORITY)

### [WONTFIX] 2.1: Extract filepath.Abs() Helper
**Risk:** LOW
**Files:** `internal/system/pathutil.go`, `internal/cli/create.go`, `internal/cli/mount.go`
**Description:** Pattern repeated 4-5 times. Add `AbsolutePath()` helper to pathutil.go.
**Rationale for WONTFIX:** Not meaningful duplication - just standard error handling of a standard library call. Go philosophy: "A little copying is better than a little dependency." The code is clearer without the abstraction. filepath.Abs() is self-documenting and widely understood.

### [ ] 2.2: Extract Authentication Method Helper
**Risk:** MEDIUM
**Files:** `internal/cli/common.go`, `internal/cli/create.go`, `internal/cli/mount.go`
**Description:** ~45 lines of duplicated auth logic. Add `GetAuthMethod()` helper to common.go.
**Benefits:** Single source of truth for auth logic.
**Testing:**
- Integration tests (keyfile auth)
- Manual: password auth for create (should prompt twice)
- Manual: password auth for mount (should prompt once)

## Phase 3: Remove Unused Code (MEDIUM PRIORITY)

### [ ] 3.1: Remove GetSudoUser()
**Risk:** VERY LOW
**Files:** `internal/system/privileges.go`
**Description:** Delete lines 21-24 (function never called).
**Verification:** `grep -r "GetSudoUser" internal/ cmd/` should show no usages.

### [ ] 3.2: Remove ParseLosetupFind()
**Risk:** LOW
**Files:** `internal/system/parser.go`
**Description:** Delete lines 60-68 (replaced by JSON-based parsing).
**Verification:** `grep -r "ParseLosetupFind" internal/ cmd/` should show no usages.

### [ ] 3.3: Remove InteractiveAuth Type
**Risk:** MEDIUM
**Files:** `internal/container/luks.go`
**Description:** Delete lines 43-52 (type definition). Simplify Format() and Open() methods to remove InteractiveAuth checks.
**Rationale:** YAGNI - no interactive mode in requirements.

### [ ] 3.4: Document PromptConfirm()
**Risk:** VERY LOW
**Files:** `internal/ui/prompt.go`
**Description:** Add comment explaining function is available for future use.
**Rationale:** Small footprint (~7 lines), likely useful for future features.

## Phase 4: Minor Improvements (LOW PRIORITY)

### [ ] 4.1: Add Permission Constants
**Risk:** VERY LOW
**Files:** Create `internal/system/permissions.go`, update `pathutil.go`, `create.go`, `mount.go`
**Description:** Replace magic numbers (0600, 0044, 0755) with named constants.
**Benefits:** Self-documenting, easier security audits.

### [ ] 4.2: Add Loop Device Comment
**Risk:** VERY LOW
**Files:** `internal/container/discovery.go`
**Description:** Add documentation explaining major number 7 is for loop devices (lines 170-177).

### [ ] 4.3: Add Minimum Size Validation
**Risk:** LOW
**Files:** `internal/cli/create.go`
**Description:** Add validation after ParseSize to require minimum 32MB container size.
**Rationale:** Prevents cryptic errors from cryptsetup/mkfs.
**Testing:**
- Valid: `brezno create test.img -s 100M` → should work
- Too small: `brezno create test.img -s 10M` → clear error message

## Implementation Order

1. Phase 4.1 - Add permission constants (safest)
2. Phase 4.2 - Add loop device comment (doc only)
3. Phase 2.1 - Extract filepath.Abs helper
4. Phase 1.1 - Fix filepath.Abs error handling (depends on 2.1)
5. Phase 1.2 - Fix cleanup error handling
6. Phase 3.1, 3.2 - Remove GetSudoUser, ParseLosetupFind
7. Phase 3.3, 3.4 - Handle InteractiveAuth and PromptConfirm
8. Phase 2.2 - Extract auth method helper (complex refactor)
9. Phase 1.3 - Fix flag shadowing (potential breaking change)
10. Phase 4.3 - Add size validation (last)

**After each phase:**
- Build: `go build -o brezno ./cmd/brezno`
- Test: `sudo ./test/integration_test.sh` (all 11 tests must pass)
- Commit: `git commit -m "refactor: Phase X.Y - <description>"`

## Notes

- All changes maintain backward compatibility
- Integration tests must pass after each phase
- Each phase should be a separate commit
- See `/home/nace/.claude/plans/zesty-bubbling-map.md` for detailed implementation plan

## Expected Benefits

**Code Quality:**
- Remove ~100 lines of unused code
- Eliminate ~50 lines of duplication
- Add proper error handling in 4 places
- Improve code documentation

**Maintainability:**
- Single source of truth for common operations
- Consistent error messages
- Self-documenting permission constants
- Better inline documentation

**Safety:**
- No ignored errors
- Minimum size validation
- Consistent cleanup handling
- Backward compatibility maintained
