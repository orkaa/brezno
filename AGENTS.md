# AGENTS.md - Context for AI Agents

> Essential context for AI agents working on the Brezno codebase.

## Project Overview

**Brezno** is a LUKS2 encrypted container management CLI — a scriptable, Linux-only alternative to VeraCrypt. It wraps standard Linux tools (cryptsetup, losetup, mount) rather than implementing its own cryptography.

- **Language**: Go 1.24 | **Platform**: Linux only | **Privileges**: All commands require root
- **Philosophy**: Containers are standard LUKS2 — usable with `cryptsetup` even without Brezno

## Architecture Principles

1. **Stateless discovery** — Never caches container state. Always queries `dmsetup` → `losetup` → `/proc/mounts` dynamically. Never add state files or caching.

2. **Wrapper pattern** — All crypto goes through `cryptsetup`/LUKS2. Never implement custom encryption.

3. **Security-first** — Passwords in `SecureBytes` (auto-zeroing), files created at `0600`, TOCTOU protection via file descriptors in resize.

4. **Dependency injection** — `GlobalContext` (internal/cli/common.go) holds all shared resources: Executor, Logger, LoopManager, LUKSManager, MountManager, Discovery.

## Project Structure

```
brezno/
├── cmd/brezno/          # Entry point, cobra root command, flag setup
└── internal/
    ├── cli/             # Command implementations (create, mount, unmount, resize, list, info, verify, backup, restore, password)
    ├── container/       # LUKS, loop device, mount, discovery logic
    ├── system/          # Executor, SecureBytes, CleanupStack, parsers, utilities
    └── ui/              # Logger, prompts, output formatting
```

## Key Patterns

### CleanupStack
RAII-style resource cleanup in LIFO order. Used in any command that allocates resources (loop devices, LUKS mappings). Pattern: create stack → `defer Execute()` → `Add()` after each allocation → `Clear()` on success. See: `internal/system/cleanup.go`, `cli/create.go`, `cli/mount.go`.

### SecureBytes
Wraps passwords with explicit zeroing and mlock (prevents swap). Always `defer pwAuth.Password.Zeroize()` immediately after obtaining auth. Never convert to `string`. See: `internal/system/secure.go`.

### GetAuthMethod
Central function for password/keyfile resolution. Signature:
```go
GetAuthMethod(keyfile string, requireConfirmation bool, promptText string, confirmText string) (container.AuthMethod, error)
```
Pass empty strings for `promptText`/`confirmText` to use defaults. Pass `requireConfirmation=true` for create/new-password operations. Automatically detects terminal vs pipe — no `--password-stdin` flag needed.

### Command execution
Always use `ctx.Executor` (not `exec.Command` directly) — provides verbose output and arg sanitization. Methods: `Run()`, `RunOutput()`, `RunCmd()`.

## Supported Filesystems

**Only**: ext4, xfs, btrfs — all three support online resize. Don't add others without online resize support.

## External Dependencies

**System tools** (checked at runtime, not all at startup):
- `cryptsetup`, `losetup`, `mount`, `umount`, `dmsetup`, `df` — checked in `CheckDependencies()`
- `blockdev` — checked in resize command
- `mkfs.{ext4,xfs,btrfs}`, `resize2fs`, `xfs_growfs`, `btrfs` — checked per-command

**Go packages**: `spf13/cobra`, `golang.org/x/term`, `golang.org/x/sys`, `fatih/color`

## Testing

Integration tests create real LUKS containers and require sudo.

```bash
sudo ./test/integration_test.sh          # all tests
sudo ./test/integration_test.sh basic    # or: resize, password, backup, restore, verify, info
```

Test files in `test/tests/`: `test_basic.sh`, `test_resize.sh`, `test_password.sh`, `test_backup.sh`, `test_restore.sh`, `test_verify.sh`, `test_info.sh`

## Adding a New Command

1. Create `internal/cli/newcmd.go`
2. Define struct with `ctx *GlobalContext` and flag fields
3. Implement `NewXxxCommand(ctx *GlobalContext) *cobra.Command`
4. Register in `cmd/brezno/main.go`

```go
type XxxCommand struct {
    ctx  *GlobalContext
    flag string
}

func NewXxxCommand(ctx *GlobalContext) *cobra.Command {
    cmd := &XxxCommand{ctx: ctx}
    cobraCmd := &cobra.Command{
        Use:  "xxx <container-path>",
        Args: cobra.MaximumNArgs(1),
        RunE: cmd.Run,
    }
    cobraCmd.Flags().StringVarP(&cmd.flag, "flag", "f", "", "Help text")
    return cobraCmd
}

func (c *XxxCommand) Run(cmd *cobra.Command, args []string) error {
    if err := system.RequireRoot(); err != nil {
        return err
    }
    if err := c.ctx.CheckDependencies(); err != nil {
        return err
    }
    // implementation
}
```

## Debugging

```bash
sudo brezno --verbose mount /tmp/test.luks /mnt/test   # shows all executed commands
sudo brezno --no-color list                             # for logs/scripts
```
