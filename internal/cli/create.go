package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nace/brezno/internal/container"
	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

// CreateCommand handles container creation
type CreateCommand struct {
	ctx        *GlobalContext
	size       string
	filesystem string
	keyfile    string
}

// NewCreateCommand creates the create command
func NewCreateCommand(ctx *GlobalContext) *cobra.Command {
	cmd := &CreateCommand{ctx: ctx}

	cobraCmd := &cobra.Command{
		Use:   "create <container-path>",
		Short: "Create a new encrypted container",
		Long:  `Create a new LUKS2 encrypted container file with the specified size and filesystem.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  cmd.Run,
	}

	cobraCmd.Flags().StringVarP(&cmd.size, "size", "s", "", "Container size (e.g., 1G, 100M)")
	cobraCmd.Flags().StringVarP(&cmd.filesystem, "filesystem", "f", "ext4", "Filesystem type (ext4, xfs, btrfs)")
	cobraCmd.Flags().StringVarP(&cmd.keyfile, "keyfile", "k", "", "Keyfile path (if not set, will prompt for passphrase)")

	return cobraCmd
}

// Run executes the create command
func (c *CreateCommand) Run(cmd *cobra.Command, args []string) error {
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

	// Convert to absolute path
	absPath, err := filepath.Abs(containerPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	containerPath = absPath

	// Get size if not provided
	if c.size == "" {
		c.size = ui.PromptString("Container size (e.g., 1G, 10G)")
	}

	// Parse size
	sizeBytes, err := system.ParseSize(c.size)
	if err != nil {
		return err
	}

	// Get filesystem (already has default)
	if c.filesystem == "" {
		c.filesystem = ui.PromptStringWithDefault("Filesystem type", "ext4")
	}

	// Validate filesystem
	if c.filesystem != "ext4" && c.filesystem != "xfs" && c.filesystem != "btrfs" {
		return fmt.Errorf("unsupported filesystem: %s (use ext4, xfs, or btrfs)", c.filesystem)
	}

	// Check if mkfs tool exists
	mkfsTool := "mkfs." + c.filesystem
	if !c.ctx.Executor.CommandExists(mkfsTool) {
		return fmt.Errorf("filesystem tool not found: %s (please install it)", mkfsTool)
	}

	// Get authentication method
	var auth container.AuthMethod
	if c.keyfile != "" {
		// Validate keyfile
		if _, err := os.Stat(c.keyfile); os.IsNotExist(err) {
			return fmt.Errorf("keyfile not found: %s", c.keyfile)
		}
		auth = &container.KeyfileAuth{KeyfilePath: c.keyfile}
	} else {
		// Prompt for password
		password, err := ui.PromptPassword("Enter passphrase")
		if err != nil {
			return fmt.Errorf("failed to read passphrase: %w", err)
		}
		defer password.Zeroize()

		confirmPassword, err := ui.PromptPassword("Confirm passphrase")
		if err != nil {
			return fmt.Errorf("failed to read passphrase: %w", err)
		}
		defer confirmPassword.Zeroize()

		if !bytes.Equal(password.Bytes(), confirmPassword.Bytes()) {
			return fmt.Errorf("passphrases don't match")
		}
		auth = &container.PasswordAuth{Password: password}
	}

	// Execute creation
	c.ctx.Logger.Info("Creating %s encrypted container: %s", system.FormatSize(sizeBytes), containerPath)
	return c.execute(containerPath, sizeBytes, auth)
}

func (c *CreateCommand) execute(path string, sizeBytes uint64, auth container.AuthMethod) error {
	cleanup := system.NewCleanupStack()
	defer func() {
		if err := cleanup.Execute(); err != nil {
			c.ctx.Logger.Warning("Cleanup errors occurred: %v", err)
		}
	}()

	// Step 1: Create sparse file with secure permissions
	c.ctx.Logger.Info("Creating sparse file...")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("file already exists: %s", path)
		}
		return fmt.Errorf("failed to create file: %w", err)
	}

	// Truncate to desired size
	if err := file.Truncate(int64(sizeBytes)); err != nil {
		file.Close()
		os.Remove(path)
		return fmt.Errorf("failed to set file size: %w", err)
	}
	file.Close()

	cleanup.Add(func() error {
		return os.Remove(path)
	})

	// Step 2: Format as LUKS
	c.ctx.Logger.Info("Formatting as LUKS2 encrypted container...")
	if err := c.ctx.LUKSManager.Format(path, auth); err != nil {
		return err
	}

	// Step 3: Attach loop device
	c.ctx.Logger.Info("Setting up loop device...")
	loopDev, err := c.ctx.LoopManager.Attach(path)
	if err != nil {
		return err
	}
	cleanup.Add(func() error {
		return c.ctx.LoopManager.Detach(loopDev)
	})

	// Step 4: Open LUKS container
	mapperName := container.GenerateMapperName(path)
	c.ctx.Logger.Info("Opening LUKS container...")
	if err := c.ctx.LUKSManager.Open(loopDev, mapperName, auth); err != nil {
		return err
	}
	cleanup.Add(func() error {
		return c.ctx.LUKSManager.Close(mapperName)
	})

	// Step 5: Create filesystem
	mapperDevice := "/dev/mapper/" + mapperName
	c.ctx.Logger.Info("Creating %s filesystem...", c.filesystem)
	if err := c.ctx.MountMgr.MakeFilesystem(mapperDevice, c.filesystem); err != nil {
		return err
	}

	// Success! Clear cleanup to prevent removal
	cleanup.Clear()

	// Clean up resources manually (close LUKS, detach loop)
	c.ctx.LUKSManager.Close(mapperName)
	c.ctx.LoopManager.Detach(loopDev)

	c.ctx.Logger.Success("Container created successfully: %s", path)
	c.ctx.Logger.Info("Size: %s, Filesystem: %s", system.FormatSize(sizeBytes), c.filesystem)

	return nil
}
