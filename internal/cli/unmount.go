package cli

import (
	"fmt"
	"path/filepath"

	"github.com/nace/brezno/internal/container"
	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

// UnmountCommand handles container unmounting
type UnmountCommand struct {
	ctx   *GlobalContext
	force bool
}

// NewUnmountCommand creates the unmount command
func NewUnmountCommand(ctx *GlobalContext) *cobra.Command {
	cmd := &UnmountCommand{ctx: ctx}

	cobraCmd := &cobra.Command{
		Use:   "unmount <container-path|mount-point|mapper-name>",
		Short: "Unmount an encrypted container",
		Long:  `Unmount a LUKS encrypted container and close all associated resources.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  cmd.Run,
	}

	cobraCmd.Flags().BoolVarP(&cmd.force, "force", "f", false, "Force unmount (try umount -f, then umount -l)")

	return cobraCmd
}

// Run executes the unmount command
func (c *UnmountCommand) Run(cmd *cobra.Command, args []string) error {
	if err := system.RequireRoot(); err != nil {
		return err
	}

	if err := c.ctx.CheckDependencies(); err != nil {
		return err
	}

	// Get identifier (path, mount point, or mapper name)
	var identifier string
	if len(args) > 0 {
		identifier = args[0]
	} else {
		identifier = ui.PromptString("Container path, mount point, or mapper name")
	}

	// Try to find the container by various methods
	var cont *container.Container
	var err error

	// Try as absolute path first
	absPath, _ := filepath.Abs(identifier)
	cont, err = c.ctx.Discovery.FindByPath(absPath)
	if err != nil {
		return err
	}

	// Try as mount point
	if cont == nil {
		cont, err = c.ctx.Discovery.FindByMount(identifier)
		if err != nil {
			return err
		}
	}

	// Try as mapper name
	if cont == nil {
		cont, err = c.ctx.Discovery.FindByMapper(identifier)
		if err != nil {
			return err
		}
	}

	if cont == nil {
		return fmt.Errorf("no mounted container found matching: %s", identifier)
	}

	// Execute unmount
	return c.execute(cont)
}

func (c *UnmountCommand) execute(cont *container.Container) error {
	// Step 1: Unmount filesystem (if mounted)
	if cont.MountPoint != "" {
		c.ctx.Logger.Info("Unmounting filesystem from %s...", cont.MountPoint)
		if err := c.ctx.MountMgr.Unmount(cont.MountPoint, c.force); err != nil {
			return fmt.Errorf("failed to unmount: %w", err)
		}
	}

	// Step 2: Close LUKS container
	if cont.MapperName != "" {
		c.ctx.Logger.Info("Closing LUKS container...")
		if err := c.ctx.LUKSManager.Close(cont.MapperName); err != nil {
			return fmt.Errorf("failed to close LUKS container: %w", err)
		}
	}

	// Step 3: Detach loop device
	if cont.LoopDevice != "" {
		c.ctx.Logger.Info("Detaching loop device...")
		if err := c.ctx.LoopManager.Detach(cont.LoopDevice); err != nil {
			// Log warning but don't fail - loop device might auto-detach
			c.ctx.Logger.Warning("Failed to detach loop device: %v", err)
		}
	}

	c.ctx.Logger.Success("Container closed successfully")
	if cont.Path != "" {
		c.ctx.Logger.Info("Container: %s", cont.Path)
	}

	return nil
}
