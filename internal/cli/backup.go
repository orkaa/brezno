package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

// BackupCommand handles LUKS header backup
type BackupCommand struct {
	ctx    *GlobalContext
	output string
	yes    bool
}

// NewBackupCommand creates the backup command
func NewBackupCommand(ctx *GlobalContext) *cobra.Command {
	cmd := &BackupCommand{ctx: ctx}

	cobraCmd := &cobra.Command{
		Use:   "backup <container-path>",
		Short: "Backup LUKS header to a file",
		Long: `Backup the LUKS header of an encrypted container to a file.

The LUKS header contains encryption metadata and key slots. If the header
is corrupted, all data in the container becomes permanently inaccessible.
Regular header backups are strongly recommended.

No password is required for header backup - it only copies encrypted metadata.`,
		Args: cobra.MaximumNArgs(1),
		RunE: cmd.Run,
	}

	cobraCmd.Flags().StringVarP(&cmd.output, "output", "o", "", "Output file path (default: <container>.header.bak)")
	cobraCmd.Flags().BoolVarP(&cmd.yes, "yes", "y", false, "Skip confirmation prompt when overwriting")

	return cobraCmd
}

// Run executes the backup command
func (c *BackupCommand) Run(cmd *cobra.Command, args []string) error {
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

	// Verify it's a LUKS container
	isLuks, err := c.ctx.LUKSManager.IsLUKS(containerPath)
	if err != nil {
		return fmt.Errorf("failed to check LUKS format: %w", err)
	}
	if !isLuks {
		return fmt.Errorf("not a LUKS container: %s", containerPath)
	}

	// Determine output path
	outputPath := c.output
	if outputPath == "" {
		outputPath = containerPath + ".header.bak"
	}

	// Resolve output path to absolute
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	// Check if output file already exists
	if _, err := os.Stat(outputPath); err == nil {
		if !c.yes {
			if !ui.PromptConfirm(fmt.Sprintf("Output file already exists: %s. Overwrite?", outputPath)) {
				return fmt.Errorf("backup cancelled")
			}
		}
		// Remove existing file to ensure clean backup
		if err := os.Remove(outputPath); err != nil {
			return fmt.Errorf("failed to remove existing backup file: %w", err)
		}
	}

	c.ctx.Logger.Info("Backing up LUKS header: %s", containerPath)
	return c.execute(containerPath, outputPath)
}

func (c *BackupCommand) execute(containerPath, outputPath string) error {
	// Backup the header (works directly on file, no loop device needed)
	c.ctx.Logger.Info("Creating header backup...")
	if err := c.ctx.LUKSManager.BackupHeader(containerPath, outputPath); err != nil {
		return err
	}

	// Set secure permissions on backup file
	if err := os.Chmod(outputPath, 0600); err != nil {
		return fmt.Errorf("failed to set backup file permissions: %w", err)
	}

	// Get backup file size for reporting
	info, err := os.Stat(outputPath)
	if err != nil {
		c.ctx.Logger.Warning("Failed to stat backup file: %v", err)
	}

	c.ctx.Logger.Success("Header backup created: %s", outputPath)
	if err == nil {
		c.ctx.Logger.Info("Backup size: %s", system.FormatSize(uint64(info.Size())))
	}

	return nil
}
