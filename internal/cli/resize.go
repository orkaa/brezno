package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nace/brezno/internal/container"
	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

// ResizeCommand handles container resizing
type ResizeCommand struct {
	ctx     *GlobalContext
	size    string
	keyfile string
	yes     bool
}

// NewResizeCommand creates the resize command
func NewResizeCommand(ctx *GlobalContext) *cobra.Command {
	cmd := &ResizeCommand{ctx: ctx}

	cobraCmd := &cobra.Command{
		Use:   "resize <container-path> [new-size]",
		Short: "Expand an encrypted container",
		Long:  `Expand a mounted LUKS2 encrypted container to a new size. The container must be mounted before resizing.`,
		Args:  cobra.RangeArgs(1, 2),
		RunE:  cmd.Run,
	}

	cobraCmd.Flags().StringVarP(&cmd.size, "size", "s", "", "New container size (e.g., 20G, 500M)")
	cobraCmd.Flags().StringVarP(&cmd.keyfile, "keyfile", "k", "", "Keyfile path for authentication")
	cobraCmd.Flags().BoolVarP(&cmd.yes, "yes", "y", false, "Skip confirmation prompt")

	return cobraCmd
}

// Run executes the resize command
func (c *ResizeCommand) Run(cmd *cobra.Command, args []string) error {
	if err := system.RequireRoot(); err != nil {
		return err
	}

	// Get container path
	containerPath := args[0]

	// Convert to absolute path and resolve symlinks
	absPath, err := filepath.Abs(containerPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Resolve symlinks to get canonical path (security: prevent symlink attacks)
	canonicalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("container not found: %s", absPath)
		}
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	containerPath = canonicalPath

	// Get size (from positional arg or flag)
	newSize := c.size
	if len(args) > 1 {
		newSize = args[1]
	}

	// Prompt for size if not provided
	if newSize == "" {
		newSize = ui.PromptString("New container size (e.g., 20G, 500M)")
	}

	// Parse size
	newSizeBytes, err := system.ParseSize(newSize)
	if err != nil {
		return err
	}

	// Execute resize
	c.ctx.Logger.Info("Resizing encrypted container: %s", containerPath)
	return c.execute(containerPath, newSizeBytes)
}

func (c *ResizeCommand) execute(containerPath string, newSizeBytes uint64) error {
	// Step 1: Open container file early to prevent TOCTOU race conditions
	// Open with O_WRONLY (we need write access for truncate)
	// We don't use O_EXCL here because the file already exists
	// Instead, we'll validate the file hasn't changed using file descriptor operations
	containerFile, err := os.OpenFile(containerPath, os.O_WRONLY, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("container not found: %s", containerPath)
		}
		return fmt.Errorf("failed to open container: %w", err)
	}
	defer containerFile.Close()

	// Get file info using the file descriptor (not the path) to ensure we're checking the right file
	containerInfo, err := containerFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat container file: %w", err)
	}

	// Validate it's a regular file
	if !containerInfo.Mode().IsRegular() {
		return fmt.Errorf("container must be a regular file, not a directory or device: %s", containerPath)
	}

	// Step 2: Find the active container using Discovery
	c.ctx.Logger.Info("Checking container status...")
	activeContainer, err := c.ctx.Discovery.FindByPath(containerPath)
	if err != nil {
		return fmt.Errorf("failed to discover container: %w", err)
	}

	// Step 3: Validate container is mounted
	if activeContainer == nil {
		return fmt.Errorf("container must be mounted for resize. Use 'brezno mount' first")
	}

	if activeContainer.MountPoint == "" {
		return fmt.Errorf("container is open but not mounted. Please mount it first")
	}

	if activeContainer.Filesystem == "" {
		return fmt.Errorf("cannot detect filesystem type")
	}

	// Step 4: Check filesystem-specific resize tool exists
	if !c.ctx.Executor.CommandExists("blockdev") {
		return fmt.Errorf("blockdev not found (install util-linux)")
	}

	switch activeContainer.Filesystem {
	case "ext4":
		if !c.ctx.Executor.CommandExists("resize2fs") {
			return fmt.Errorf("resize2fs not found (install e2fsprogs)")
		}
	case "xfs":
		if !c.ctx.Executor.CommandExists("xfs_growfs") {
			return fmt.Errorf("xfs_growfs not found (install xfsprogs)")
		}
	case "btrfs":
		if !c.ctx.Executor.CommandExists("btrfs") {
			return fmt.Errorf("btrfs not found (install btrfs-progs)")
		}
	default:
		return fmt.Errorf("filesystem %s does not support online resize", activeContainer.Filesystem)
	}

	// Step 5: Get current sizes
	// Use file descriptor to get size (already have containerInfo from Stat above)
	currentFileSize := uint64(containerInfo.Size())

	currentFSSize, currentFSUsed, err := c.ctx.MountMgr.GetFilesystemSize(activeContainer.MountPoint)
	if err != nil {
		return fmt.Errorf("failed to get filesystem size: %w", err)
	}

	// Step 6: Validate new size is larger
	if newSizeBytes <= currentFileSize {
		return fmt.Errorf("new size (%s) must be larger than current size (%s)",
			system.FormatSize(newSizeBytes), system.FormatSize(currentFileSize))
	}

	// Step 7: Check available disk space
	expansionBytes := newSizeBytes - currentFileSize
	availableSpace, err := system.GetAvailableSpace(containerPath)
	if err != nil {
		c.ctx.Logger.Warning("Failed to check available disk space: %v", err)
	} else if expansionBytes > availableSpace {
		return fmt.Errorf("insufficient disk space: need %s, available %s",
			system.FormatSize(expansionBytes), system.FormatSize(availableSpace))
	}

	// Step 8: Show preview and get confirmation
	c.ctx.Logger.Info("Container: %s", containerPath)
	c.ctx.Logger.Info("Mount point: %s", activeContainer.MountPoint)
	c.ctx.Logger.Info("Filesystem: %s", activeContainer.Filesystem)
	c.ctx.Logger.Info("")
	c.ctx.Logger.Info("Current size: %s", system.FormatSize(currentFileSize))
	c.ctx.Logger.Info("New size:     %s", system.FormatSize(newSizeBytes))
	c.ctx.Logger.Info("Expansion:    %s", system.FormatSize(expansionBytes))
	c.ctx.Logger.Info("")
	c.ctx.Logger.Info("Filesystem used: %s of %s", system.FormatSize(currentFSUsed), system.FormatSize(currentFSSize))

	if !c.yes {
		if !ui.PromptConfirm("Proceed with resize?") {
			return fmt.Errorf("resize cancelled by user")
		}
	}

	// Step 9: Get authentication
	auth, err := GetAuthMethod(c.keyfile, false) // false = no confirmation needed
	if err != nil {
		return err
	}
	// Ensure password is zeroized when done
	if pwAuth, ok := auth.(*container.PasswordAuth); ok {
		defer pwAuth.Password.Zeroize()
	}

	// Step 10: Perform resize operations
	// Note: No CleanupStack needed - resize operations are monotonic (safe if interrupted)

	// Step 10a: Expand container file using the already-open file descriptor
	// This prevents TOCTOU race conditions
	c.ctx.Logger.Info("Expanding container file...")
	if err := containerFile.Truncate(int64(newSizeBytes)); err != nil {
		return fmt.Errorf("failed to expand container file: %w", err)
	}
	// Sync to ensure changes are written to disk before proceeding
	if err := containerFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync container file: %w", err)
	}

	// Step 10b: Refresh loop device size
	c.ctx.Logger.Info("Refreshing loop device size...")
	if err := c.ctx.LoopManager.RefreshSize(activeContainer.LoopDevice); err != nil {
		c.ctx.Logger.Warning("Failed to refresh loop device (may auto-update): %v", err)
		// Continue anyway - many kernels auto-update the loop device
	}

	// Step 10c: Resize LUKS container
	c.ctx.Logger.Info("Resizing LUKS container...")
	if err := c.ctx.LUKSManager.Resize(activeContainer.MapperName, auth); err != nil {
		return fmt.Errorf("failed to resize LUKS container: %w\n"+
			"The container file has been expanded but LUKS has not.\n"+
			"You can retry: sudo brezno resize %s %s", err, containerPath, system.FormatSize(newSizeBytes))
	}

	// Step 10d: Resize filesystem
	mapperDevice := "/dev/mapper/" + activeContainer.MapperName
	c.ctx.Logger.Info("Resizing %s filesystem...", activeContainer.Filesystem)
	if err := c.ctx.MountMgr.ResizeFilesystem(mapperDevice, activeContainer.Filesystem, activeContainer.MountPoint); err != nil {
		return fmt.Errorf("failed to resize filesystem: %w\n"+
			"The LUKS container has been expanded but the filesystem has not.\n"+
			"You may need to resize manually:\n"+
			"  ext4:  sudo resize2fs %s\n"+
			"  xfs:   sudo xfs_growfs %s\n"+
			"  btrfs: sudo btrfs filesystem resize max %s",
			err, mapperDevice, activeContainer.MountPoint, activeContainer.MountPoint)
	}

	// Step 11: Verify success
	newFSSize, newFSUsed, err := c.ctx.MountMgr.GetFilesystemSize(activeContainer.MountPoint)
	if err != nil {
		c.ctx.Logger.Warning("Failed to verify new filesystem size: %v", err)
	}

	c.ctx.Logger.Success("Container resized successfully!")
	c.ctx.Logger.Info("Old size: %s → New size: %s", system.FormatSize(currentFSSize), system.FormatSize(newFSSize))
	c.ctx.Logger.Info("Used: %s, Available: %s", system.FormatSize(newFSUsed), system.FormatSize(newFSSize-newFSUsed))

	return nil
}
