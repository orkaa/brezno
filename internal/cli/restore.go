package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

// RestoreCommand handles LUKS header restore
type RestoreCommand struct {
	ctx    *GlobalContext
	backup string
	yes    bool
}

// NewRestoreCommand creates the restore command
func NewRestoreCommand(ctx *GlobalContext) *cobra.Command {
	cmd := &RestoreCommand{ctx: ctx}

	cobraCmd := &cobra.Command{
		Use:   "restore <container-path>",
		Short: "Restore LUKS header from a backup file",
		Long: `Restore the LUKS header of an encrypted container from a backup file.

WARNING: This is a dangerous operation. Restoring the wrong header will make
your data permanently inaccessible. Only restore from a backup of the same
container.

The container must NOT be mounted during restore. No password is required
for header restore - it only writes encrypted metadata.`,
		Args: cobra.MaximumNArgs(1),
		RunE: cmd.Run,
	}

	cobraCmd.Flags().StringVarP(&cmd.backup, "backup", "b", "", "Backup file path (default: <container>.header.bak)")
	cobraCmd.Flags().BoolVarP(&cmd.yes, "yes", "y", false, "Skip confirmation prompt (use with caution!)")

	return cobraCmd
}

// Run executes the restore command
func (c *RestoreCommand) Run(cmd *cobra.Command, args []string) error {
	if err := system.RequireRoot(); err != nil {
		return err
	}

	if err := c.ctx.CheckDependencies(); err != nil {
		return err
	}

	// Get container path
	var containerPath string
	if len(args) > 0 {
		containerPath = args[0]
	} else {
		containerPath = ui.PromptString("Container file path")
	}

	// Resolve to absolute path and follow symlinks
	absPath, err := filepath.Abs(containerPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	containerPath = resolvedPath

	// Verify container exists
	if _, err := os.Stat(containerPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("container not found: %s", containerPath)
		}
		return fmt.Errorf("failed to access container: %w", err)
	}

	// Determine backup path
	backupPath := c.backup
	if backupPath == "" {
		backupPath = containerPath + ".header.bak"
	}

	// Resolve backup path to absolute
	backupPath, err = filepath.Abs(backupPath)
	if err != nil {
		return fmt.Errorf("invalid backup path: %w", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("backup file not found: %s", backupPath)
		}
		return fmt.Errorf("failed to access backup file: %w", err)
	}

	// Verify backup is a valid LUKS header
	isLuks, err := c.ctx.LUKSManager.IsLUKS(backupPath)
	if err != nil {
		return fmt.Errorf("failed to verify backup file: %w", err)
	}
	if !isLuks {
		return fmt.Errorf("backup file is not a valid LUKS header: %s", backupPath)
	}

	// Check container is NOT mounted (critical safety check)
	container, err := c.ctx.Discovery.FindByPath(containerPath)
	if err != nil {
		return fmt.Errorf("failed to check container state: %w", err)
	}
	if container != nil {
		return fmt.Errorf("container is currently mounted - unmount before restoring header")
	}

	// Display warning and require confirmation
	if !c.yes {
		c.ctx.Logger.Warning("You are about to restore the LUKS header of: %s", containerPath)
		c.ctx.Logger.Warning("From backup: %s", backupPath)
		c.ctx.Logger.Warning("")
		c.ctx.Logger.Warning("WARNING: Restoring the wrong header will make your data")
		c.ctx.Logger.Warning("PERMANENTLY INACCESSIBLE. Only proceed if this backup")
		c.ctx.Logger.Warning("was created from the same container.")
		c.ctx.Logger.Warning("")

		if !ui.PromptConfirm("Are you sure you want to restore this header?") {
			return fmt.Errorf("restore cancelled")
		}
	}

	c.ctx.Logger.Info("Restoring LUKS header: %s", containerPath)
	return c.execute(containerPath, backupPath)
}

func (c *RestoreCommand) execute(containerPath, backupPath string) error {
	c.ctx.Logger.Info("Restoring header from backup...")
	if err := c.ctx.LUKSManager.RestoreHeader(containerPath, backupPath); err != nil {
		return err
	}

	c.ctx.Logger.Success("Header restored successfully: %s", containerPath)
	c.ctx.Logger.Info("Backup used: %s", backupPath)

	return nil
}
