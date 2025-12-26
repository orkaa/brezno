# AGENTS.md - Context for AI Agents

> Essential context for AI agents working on the Brezno codebase. This document helps you understand the project quickly, follow established patterns, and avoid common mistakes.

## Project Overview

**Brezno** is a LUKS2 encrypted container management CLI tool - essentially a command-line alternative to VeraCrypt for Linux. It provides a simple, scriptable interface for creating, mounting, resizing, and managing encrypted containers.

- **Language**: Go 1.24
- **Platform**: Linux only (requires dm-crypt kernel support)
- **Privileges**: All operations require root
- **Philosophy**: Wraps standard Linux tools rather than reimplementing cryptography

## Core Architecture Principles

### 1. Stateless Discovery
Brezno **never caches container state**. It always queries system state dynamically by correlating:
- `dmsetup` → LUKS mappers
- `losetup` → loop devices and backing files
- `/proc/mounts` → mount points and filesystems

**Why this matters**: This design prevents state drift and ensures accuracy. Never add caching or state files.

### 2. Wrapper Pattern
Brezno wraps standard Linux tools (cryptsetup, losetup, mount) rather than implementing custom cryptography. This ensures:
- Security through well-audited tools (dm-crypt/LUKS2)
- Portability (containers work with standard tools)
- Maintainability (no custom crypto to audit)

### 3. Security-First Design
- **SecureBytes**: Automatic memory zeroing for passwords
- **Command Sanitization**: Redacts sensitive args in logs
- **TOCTOU Protection**: File descriptor-based operations
- **Permissions**: Container files created with 0600

### 4. Dependency Injection
`GlobalContext` (internal/cli/common.go) holds shared resources:
- Executor, Logger
- LoopManager, LUKSManager, MountManager, Discovery

## Project Structure

```
brezno/
├── cmd/brezno/          # Main entry point
└── internal/
    ├── cli/             # Command implementations (create, mount, unmount, resize, list)
    ├── container/       # LUKS, loop device, mount, discovery logic
    ├── system/          # Executor, secure bytes, cleanup, parsers, utilities
    └── ui/              # Logger, prompts, output formatting
```

**Total codebase**: ~2,270 lines of Go (very focused and compact)

## Critical Code Patterns

### CleanupStack Pattern

The CleanupStack provides RAII-style cleanup in LIFO order, similar to bash traps. Use it for **any operation that allocates resources** (loop devices, LUKS mappings, mounts).

**Implementation** (internal/system/cleanup.go):
```go
type CleanupStack struct {
    cleanups []func() error
    mu       sync.Mutex
}

func (s *CleanupStack) Add(cleanup func() error) {
    s.cleanups = append(s.cleanups, cleanup)
}

func (s *CleanupStack) Execute() error {
    // Execute in reverse order (LIFO)
    for i := len(s.cleanups) - 1; i >= 0; i-- {
        if err := s.cleanups[i](); err != nil {
            errs = append(errs, err)
        }
    }
    return /* combined errors */
}

func (s *CleanupStack) Clear() {
    s.cleanups = nil  // Success - prevent cleanup
}
```

**Usage Example** (internal/cli/create.go:109-137):
```go
func (c *CreateCommand) execute(path string, sizeBytes uint64, auth container.AuthMethod) error {
    cleanup := system.NewCleanupStack()
    defer func() {
        if err := cleanup.Execute(); err != nil {
            c.ctx.Logger.Warning("Cleanup errors occurred: %v", err)
        }
    }()

    // Step 1: Create file
    file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
    if err != nil {
        return err
    }
    file.Truncate(int64(sizeBytes))
    file.Close()

    cleanup.Add(func() error {
        return os.Remove(path)  // Remove on failure
    })

    // Step 2: Attach loop device
    loopDev, err := c.ctx.LoopManager.Attach(path)
    if err != nil {
        return err  // Cleanup will remove file
    }
    cleanup.Add(func() error {
        return c.ctx.LoopManager.Detach(loopDev)  // Detach on failure
    })

    // ... more steps with cleanup.Add() for each resource ...

    // Success! Clear cleanup to prevent removal
    cleanup.Clear()

    return nil
}
```

**Key Points**:
- Create CleanupStack at function start
- Defer Execute() to run on any return/panic
- Add cleanup after each resource allocation
- Clear() on success to prevent cleanup
- Cleanup runs in reverse order (LIFO)

### SecureBytes Pattern

SecureBytes protects sensitive data (passwords) with automatic memory zeroing.

**Implementation** (internal/system/secure.go):
```go
type SecureBytes struct {
    data []byte
}

func NewSecureBytes(data []byte) *SecureBytes {
    sb := &SecureBytes{data: data}

    // Finalizer zeros memory on garbage collection
    runtime.SetFinalizer(sb, func(s *SecureBytes) {
        s.Zeroize()
    })

    return sb
}

func (s *SecureBytes) Zeroize() {
    for i := range s.data {
        s.data[i] = 0  // Zero memory
    }
    s.data = nil
}

func (s *SecureBytes) Bytes() []byte {
    return s.data  // Caller should not retain this
}
```

**Usage Example** (internal/cli/create.go:94-101):
```go
// Get authentication method (password or keyfile)
auth, err := GetAuthMethod(c.keyfile, true)
if err != nil {
    return err
}

// Ensure password is zeroized when function returns
if pwAuth, ok := auth.(*container.PasswordAuth); ok {
    defer pwAuth.Password.Zeroize()
}

// ... use auth for LUKS operations ...
```

**PasswordAuth Usage** (internal/container/luks.go:17-29):
```go
type PasswordAuth struct {
    Password *system.SecureBytes
}

func (a *PasswordAuth) Apply(cmd *exec.Cmd) error {
    if a.Password == nil {
        return fmt.Errorf("password is nil")
    }
    // Use bytes.NewBuffer to avoid string conversion
    // (strings are immutable and can't be zeroized)
    cmd.Stdin = bytes.NewBuffer(append(a.Password.Bytes(), '\n'))
    return nil
}
```

**Key Points**:
- Use SecureBytes for ALL passwords/sensitive data
- Always defer Zeroize() after obtaining password
- Never convert to string (strings can't be zeroized)
- Finalizer provides safety net, but explicit Zeroize() is better

### Error Handling Pattern

Always return errors with context wrapping. Never panic in production code.

```go
// ✅ Good - explicit error with context
if err := c.ctx.LUKSManager.Format(path, auth); err != nil {
    return fmt.Errorf("failed to format LUKS container: %w", err)
}

// ✅ Good - check specific error types when needed
if os.IsExist(err) {
    return fmt.Errorf("file already exists: %s", path)
}

// ❌ Bad - swallowing errors
c.ctx.LUKSManager.Format(path, auth)  // ignoring error

// ❌ Bad - panic in production code
if err != nil {
    panic(err)  // Never do this!
}
```

### Command Execution Pattern

Use `ctx.Executor` for all command execution (provides debug output and sanitization).

```go
// For simple commands
err := m.executor.Run("cryptsetup", "luksClose", mapperName)

// For commands needing output
output, err := m.executor.RunOutput("blockdev", "--getsize64", device)

// For commands needing custom stdin (auth, etc.)
cmd := exec.Command("cryptsetup", "luksOpen", device, mapperName)
if err := auth.Apply(cmd); err != nil {
    return err
}
_, err := m.executor.RunCmd(cmd)
```

The executor automatically sanitizes output (redacts keyfile paths, stdin markers).

## Discovery Mechanism

Brezno discovers active containers by querying system state:

1. **Query dmsetup** → Find all crypt mappers (LUKS containers)
2. **Parse mapper tables** → Extract backing loop device for each mapper
3. **Query losetup** → Find backing file for each loop device
4. **Check /proc/mounts** → Find mount point and filesystem info

This happens fresh on every operation - **no caching**.

**Why no caching?** System state can change externally (manual cryptsetup/mount commands, other tools, crashes). Always querying ensures accuracy and prevents bugs from stale state.

## Important Design Decisions (Don't Break These!)

### ✅ Stateless Discovery
- Never add caching or state files
- Always query system state dynamically
- Discovery code: internal/container/discovery.go

### ✅ Mapper Name Generation
- Mapper names generated consistently from container path
- See: `container.GenerateMapperName(path)`
- Must be deterministic for discovery to work

### ✅ Loop Device Backing Paths
- Loop device backing file paths are always absolute (kernel guarantee)
- Never need to resolve relative paths in discovery

### ✅ Supported Filesystems
- **Only**: ext4, xfs, btrfs
- Why? All three support online resizing
- Don't add filesystems without online resize support

### ✅ All Operations Require Root
- Check with `system.RequireRoot()` at command start
- No privilege escalation - fail early with clear message

## Security Considerations

When making changes, ensure:

### Password Security
```go
// ✅ Use SecureBytes
password := system.NewSecureBytes([]byte(userInput))
defer password.Zeroize()

// ❌ Never use plain strings
password := "secretpass123"  // Can't be zeroized!
```

### File Permissions
```go
// ✅ Secure permissions for containers
file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
```

### TOCTOU Protection
```go
// ✅ File descriptor approach (resize.go:108)
file, err := os.OpenFile(containerPath, os.O_RDWR, 0)
defer file.Close()
// Work with file descriptor, not path
```

### Command Sanitization
- Executor automatically redacts sensitive args
- Never log passwords or keyfile contents
- See: internal/system/executor.go sanitization logic

### No Custom Crypto
- ❌ Never implement custom encryption
- ✅ Always use cryptsetup/LUKS2
- Brezno is a *wrapper*, not a crypto implementation

## External Dependencies

### System Tools (Required)
- **cryptsetup** - LUKS operations
- **losetup** - Loop device management
- **mount/umount** - Filesystem mounting
- **dmsetup** - Device mapper queries
- **df** - Disk usage
- **blockdev** - Block device queries
- **mkfs.{ext4,xfs,btrfs}** - Filesystem creation
- **resize2fs** - ext4 resizing
- **xfs_growfs** - XFS resizing
- **btrfs** - btrfs resizing

Dependency checks: `GlobalContext.CheckDependencies()`

### Go Dependencies (Minimal)
- **spf13/cobra** - CLI framework
- **golang.org/x/term** - Password prompts (no echo)
- **fatih/color** - Colored terminal output

## Testing Guidelines

### Integration Tests
- Location: `test/`
- **Require sudo**: Tests create actual LUKS containers
- Run with: `sudo ./test/integration_test.sh`
- Tests are independent (run in any order)
- Each test cleans up its containers

### Test Structure
```bash
test/
├── integration_test.sh       # Main test runner
├── basic_tests.sh             # Core functionality tests
└── resize_tests.sh            # Resize-specific tests
```

### Running Tests
```bash
# All tests
sudo ./test/integration_test.sh

# Specific test suite
sudo ./test/integration_test.sh basic_tests
sudo ./test/integration_test.sh resize_tests
```

## Common Pitfalls

### ❌ DON'T: Cache container state
```go
// BAD - state can become stale
var cachedContainers []*Container
```

### ❌ DON'T: Forget CleanupStack
```go
// BAD - resources leak on error
loopDev, _ := loopMgr.Attach(path)
// ... error occurs, loop device not cleaned up
```

### ❌ DON'T: Log passwords
```go
// BAD - password in logs
log.Printf("Using password: %s", password)
```

### ❌ DON'T: Hard-code filesystem types
```go
// BAD - not extensible
if filesystem != "ext4" {
    return errors.New("only ext4 supported")
}

// GOOD - support all three
if filesystem != "ext4" && filesystem != "xfs" && filesystem != "btrfs" {
    return fmt.Errorf("unsupported filesystem: %s", filesystem)
}
```

### ❌ DON'T: Assume container locations
```go
// BAD - containers can be anywhere
containerDir := "/var/lib/brezno/containers"

// GOOD - accept any path
containerPath = filepath.Abs(userProvidedPath)
```

### ❌ DON'T: Implement custom crypto
```go
// BAD - never do this
func encryptData(data []byte) []byte {
    // custom encryption logic
}

// GOOD - use cryptsetup
executor.Run("cryptsetup", "luksFormat", ...)
```

## Development Tips

### Adding a New Command

1. Create file in `internal/cli/` (e.g., `newcmd.go`)
2. Define command struct with `ctx *GlobalContext`
3. Implement `New<Command>Command(ctx *GlobalContext) *cobra.Command`
4. Add command to root in `cmd/brezno/main.go`

**Example skeleton**:
```go
type NewCommand struct {
    ctx *GlobalContext
    // flags...
}

func NewNewCommand(ctx *GlobalContext) *cobra.Command {
    cmd := &NewCommand{ctx: ctx}

    cobraCmd := &cobra.Command{
        Use:   "newcmd <args>",
        Short: "Description",
        Args:  cobra.MaximumNArgs(1),
        RunE:  cmd.Run,
    }

    cobraCmd.Flags().StringVarP(&cmd.flag, "flag", "f", "", "Help")
    return cobraCmd
}

func (c *NewCommand) Run(cmd *cobra.Command, args []string) error {
    if err := system.RequireRoot(); err != nil {
        return err
    }
    // ... implementation ...
}
```

### Debugging

```bash
# Verbose output (shows all commands)
./brezno --verbose create /tmp/test.luks -s 100M

# Dry-run mode (shows commands without executing)
./brezno --dry-run create /tmp/test.luks -s 100M

# No color (for logs/scripts)
./brezno --no-color list
```

### Looking for Patterns

- **CleanupStack usage**: See create.go, mount.go
- **SecureBytes usage**: See cli/common.go GetAuthMethod()
- **Discovery logic**: See container/discovery.go
- **Command execution**: See container/luks.go, loop.go, mount.go
- **Error handling**: Throughout - use grep for "fmt.Errorf"

## Quick Reference

```bash
# Build
go build -o brezno ./cmd/brezno

# Install
sudo cp brezno /usr/local/bin/

# Test
sudo ./test/integration_test.sh

# Format code
go fmt ./...

# Lint
go vet ./...
```

## Future Roadmap

Features on the roadmap (not yet implemented):

- `brezno password` - Change container password/keyfile
- `brezno backup` - Backup LUKS header
- `brezno verify` - Verify container integrity
- `brezno info` - Show detailed container information

When implementing these, follow the existing patterns (CleanupStack, SecureBytes, stateless discovery).

---

## Key Takeaways

1. **Security First**: SecureBytes for passwords, no custom crypto, secure permissions
2. **Stateless Always**: Query system state, never cache
3. **Clean Resources**: Use CleanupStack for all resource allocation
4. **Wrap, Don't Reimplement**: Use cryptsetup, losetup, mount
5. **Fail Fast**: Require root early, check dependencies, validate inputs
6. **Test Everything**: Integration tests require sudo, test both success and failure paths

When in doubt, look at existing commands (create, mount, resize) for patterns to follow.
