package cli

import (
	"fmt"
	"path/filepath"

	"github.com/nace/brezno/internal/container"
	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

// MountCommand handles container mounting
type MountCommand struct {
	ctx           *GlobalContext
	keyfile       string
	readonly      bool
	passwordStdin bool
}

// NewMountCommand creates the mount command
func NewMountCommand(ctx *GlobalContext) *cobra.Command {
	cmd := &MountCommand{ctx: ctx}

	cobraCmd := &cobra.Command{
		Use:   "mount <container-path> <mount-point>",
		Short: "Mount an encrypted container",
		Long:  `Open a LUKS encrypted container and mount its filesystem.`,
		Args:  cobra.MaximumNArgs(2),
		RunE:  cmd.Run,
	}

	cobraCmd.Flags().StringVarP(&cmd.keyfile, "keyfile", "k", "", "Keyfile path (if not set, will prompt for password)")
	cobraCmd.Flags().BoolVarP(&cmd.readonly, "readonly", "r", false, "Mount as read-only")
	cobraCmd.Flags().BoolVar(&cmd.passwordStdin, "password-stdin", false, "Read password from stdin (for automation)")

	return cobraCmd
}

// Run executes the mount command
func (c *MountCommand) Run(cmd *cobra.Command, args []string) error {
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
	c.ctx.Logger.Debug("Resolved container path: %s", containerPath)

	// Verify it's a LUKS container (will fail if file doesn't exist)
	c.ctx.Logger.Debug("Checking if %s is a LUKS container", containerPath)
	isLuks, err := c.ctx.LUKSManager.IsLUKS(containerPath)
	if err != nil {
		return fmt.Errorf("failed to check LUKS format: %w", err)
	}
	if !isLuks {
		return fmt.Errorf("not a LUKS container: %s", containerPath)
	}

	// Check if already mounted
	existing, err := c.ctx.Discovery.FindByPath(containerPath)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("container already mounted at: %s", existing.MountPoint)
	}

	// Get mount point
	var mountPoint string
	if len(args) > 1 {
		mountPoint = args[1]
	} else {
		mountPoint = ui.PromptString("Mount point")
	}

	// Convert to absolute path
	absMount, err := filepath.Abs(mountPoint)
	if err != nil {
		return fmt.Errorf("invalid mount point: %w", err)
	}
	mountPoint = absMount

	// Get authentication method
	auth, err := GetAuthMethod(c.keyfile, false, c.passwordStdin, "", "") // false = no password confirmation
	if err != nil {
		return err
	}
	// Ensure password is zeroized when done
	if pwAuth, ok := auth.(*container.PasswordAuth); ok {
		defer pwAuth.Password.Zeroize()
	}

	// Execute mount
	return c.execute(containerPath, mountPoint, auth)
}

func (c *MountCommand) execute(path, mountPoint string, auth container.AuthMethod) error {
	cleanup := system.NewCleanupStack()
	defer func() {
		if err := cleanup.Execute(); err != nil {
			c.ctx.Logger.Warning("Cleanup errors occurred: %v", err)
		}
	}()

	// Step 1: Attach loop device
	c.ctx.Logger.Info("Setting up loop device...")
	loopDev, err := c.ctx.LoopManager.Attach(path)
	if err != nil {
		return err
	}
	cleanup.Add(func() error {
		return c.ctx.LoopManager.Detach(loopDev)
	})

	// Step 2: Open LUKS container
	mapperName := container.GenerateMapperName(path)
	c.ctx.Logger.Info("Opening LUKS container...")
	if err := c.ctx.LUKSManager.Open(loopDev, mapperName, auth); err != nil {
		return err
	}
	cleanup.Add(func() error {
		return c.ctx.LUKSManager.Close(mapperName)
	})

	// Step 3: Mount filesystem
	mapperDevice := "/dev/mapper/" + mapperName
	c.ctx.Logger.Info("Mounting filesystem...")
	if err := c.ctx.MountMgr.Mount(mapperDevice, mountPoint, c.readonly); err != nil {
		return err
	}

	// Success! Clear cleanup
	cleanup.Clear()

	c.ctx.Logger.Success("Container mounted at: %s", mountPoint)

	return nil
}
