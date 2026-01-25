package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

// InfoResult contains comprehensive container information
type InfoResult struct {
	// Container file info
	ContainerPath string `json:"container_path"`
	FileSize      uint64 `json:"file_size_bytes"`
	Permissions   string `json:"permissions,omitempty"`
	ModifiedTime  string `json:"modified_time,omitempty"`

	// LUKS header metadata
	LUKSVersion string `json:"luks_version,omitempty"`
	UUID        string `json:"uuid,omitempty"`
	Cipher      string `json:"cipher,omitempty"`
	CipherMode  string `json:"cipher_mode,omitempty"`
	HashSpec    string `json:"hash_spec,omitempty"`
	KeySlots    []int  `json:"key_slots,omitempty"`

	// Runtime status
	IsActive   bool   `json:"is_active"`
	MapperName string `json:"mapper_name,omitempty"`
	LoopDevice string `json:"loop_device,omitempty"`
	MountPoint string `json:"mount_point,omitempty"`
	Filesystem string `json:"filesystem,omitempty"`

	// Disk usage (only if mounted)
	TotalSize uint64 `json:"total_size_bytes,omitempty"`
	UsedSize  uint64 `json:"used_size_bytes,omitempty"`
	Available uint64 `json:"available_bytes,omitempty"`

	// Errors
	Errors []string `json:"errors,omitempty"`
}

// InfoCommand handles displaying container information
type InfoCommand struct {
	ctx        *GlobalContext
	jsonOutput bool
}

// NewInfoCommand creates the info command
func NewInfoCommand(ctx *GlobalContext) *cobra.Command {
	cmd := &InfoCommand{ctx: ctx}

	cobraCmd := &cobra.Command{
		Use:   "info <container-path>",
		Short: "Display comprehensive container information",
		Long: `Display comprehensive information about a LUKS encrypted container.

Shows:
  - File properties (size, permissions, modification time)
  - LUKS header metadata (version, UUID, cipher, key slots)
  - Mount status (if active: mapper, loop device, mount point, filesystem)
  - Disk usage (if mounted: total, used, available)

This command does not require authentication and works on both mounted
and unmounted containers.`,
		Args: cobra.MaximumNArgs(1),
		RunE: cmd.Run,
	}

	cobraCmd.Flags().BoolVarP(&cmd.jsonOutput, "json", "j", false, "Output results in JSON format")

	return cobraCmd
}

// Run executes the info command
func (c *InfoCommand) Run(cmd *cobra.Command, args []string) error {
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
		if c.jsonOutput {
			return fmt.Errorf("container path is required in JSON mode")
		}
		containerPath = ui.PromptString("Container file path")
	}

	// Resolve to absolute path and follow symlinks
	absPath, err := filepath.Abs(containerPath)
	if err != nil {
		return c.outputError(containerPath, fmt.Errorf("invalid path: %w", err))
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return c.outputError(containerPath, fmt.Errorf("failed to resolve path: %w", err))
	}
	containerPath = resolvedPath

	// Verify container exists
	if _, err := os.Stat(containerPath); err != nil {
		if os.IsNotExist(err) {
			return c.outputError(containerPath, fmt.Errorf("container not found: %s", containerPath))
		}
		return c.outputError(containerPath, fmt.Errorf("failed to access container: %w", err))
	}

	if !c.jsonOutput {
		c.ctx.Logger.Info("Gathering information for: %s", containerPath)
	}

	return c.execute(containerPath)
}

func (c *InfoCommand) execute(containerPath string) error {
	result := &InfoResult{
		ContainerPath: containerPath,
	}

	// Step 1: Get file information
	fileInfo, err := os.Stat(containerPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to get file info: %v", err))
	} else {
		result.FileSize = uint64(fileInfo.Size())
		result.Permissions = fmt.Sprintf("%04o", fileInfo.Mode().Perm())
		result.ModifiedTime = fileInfo.ModTime().Format("2006-01-02 15:04:05")
	}

	// Step 2: Check if it's a LUKS container and get header info
	isLuks, err := c.ctx.LUKSManager.IsLUKS(containerPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to check LUKS format: %v", err))
	} else if !isLuks {
		result.Errors = append(result.Errors, "not a valid LUKS container")
	} else {
		// Get LUKS header metadata
		info, err := c.ctx.LUKSManager.Dump(containerPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to read LUKS header: %v", err))
		} else {
			result.LUKSVersion = info.Version
			result.UUID = info.UUID
			result.Cipher = info.Cipher
			result.CipherMode = info.CipherMode
			result.HashSpec = info.HashSpec
			result.KeySlots = info.KeySlots
		}
	}

	// Step 3: Check if container is active (mounted)
	activeContainer, err := c.ctx.Discovery.FindByPath(containerPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to check mount status: %v", err))
	} else if activeContainer != nil {
		result.IsActive = true
		result.MapperName = activeContainer.MapperName
		result.LoopDevice = activeContainer.LoopDevice
		result.MountPoint = activeContainer.MountPoint
		result.Filesystem = activeContainer.Filesystem

		// Step 4: Get disk usage if mounted
		if activeContainer.MountPoint != "" {
			result.TotalSize = activeContainer.Size
			result.UsedSize = activeContainer.Used
			if activeContainer.Size > activeContainer.Used {
				result.Available = activeContainer.Size - activeContainer.Used
			}
		}
	}

	return c.output(result)
}

func (c *InfoCommand) output(result *InfoResult) error {
	if c.jsonOutput {
		return c.outputJSON(result)
	}
	return c.outputHuman(result)
}

func (c *InfoCommand) outputJSON(result *InfoResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	// Return error if there were any errors
	if len(result.Errors) > 0 {
		return fmt.Errorf("info gathering had errors")
	}
	return nil
}

func (c *InfoCommand) outputHuman(result *InfoResult) error {
	// Section 1: Container file info
	c.ctx.Logger.Info("")
	c.ctx.Logger.Info("Container: %s", result.ContainerPath)
	if result.FileSize > 0 {
		c.ctx.Logger.Info("  File Size: %s", system.FormatSize(result.FileSize))
	}
	if result.Permissions != "" {
		c.ctx.Logger.Info("  Permissions: %s", result.Permissions)
	}
	if result.ModifiedTime != "" {
		c.ctx.Logger.Info("  Modified: %s", result.ModifiedTime)
	}

	// Section 2: LUKS header (if valid)
	if result.LUKSVersion != "" {
		c.ctx.Logger.Info("")
		c.ctx.Logger.Info("LUKS Header:")
		c.ctx.Logger.Info("  Version: LUKS%s", result.LUKSVersion)
		if result.UUID != "" {
			c.ctx.Logger.Info("  UUID: %s", result.UUID)
		}
		if result.Cipher != "" && result.CipherMode != "" {
			c.ctx.Logger.Info("  Cipher: %s-%s", result.Cipher, result.CipherMode)
		}
		if result.HashSpec != "" {
			c.ctx.Logger.Info("  Hash: %s", result.HashSpec)
		}
		if len(result.KeySlots) > 0 {
			slots := make([]string, len(result.KeySlots))
			for i, slot := range result.KeySlots {
				slots[i] = fmt.Sprintf("%d", slot)
			}
			c.ctx.Logger.Info("  Active Keyslots: %s", strings.Join(slots, ", "))
		}
	}

	// Section 3: Status
	c.ctx.Logger.Info("")
	if result.IsActive {
		c.ctx.Logger.Info("Status: Mounted")
		if result.MapperName != "" {
			c.ctx.Logger.Info("  Mapper: /dev/mapper/%s", result.MapperName)
		}
		if result.LoopDevice != "" {
			c.ctx.Logger.Info("  Loop Device: %s", result.LoopDevice)
		}
		if result.MountPoint != "" {
			c.ctx.Logger.Info("  Mount Point: %s", result.MountPoint)
		}
		if result.Filesystem != "" {
			c.ctx.Logger.Info("  Filesystem: %s", result.Filesystem)
		}

		// Section 4: Disk usage (if mounted)
		if result.MountPoint != "" && result.TotalSize > 0 {
			c.ctx.Logger.Info("")
			c.ctx.Logger.Info("Disk Usage:")
			c.ctx.Logger.Info("  Total: %s", system.FormatSize(result.TotalSize))

			percentage := float64(0)
			if result.TotalSize > 0 {
				percentage = float64(result.UsedSize) / float64(result.TotalSize) * 100
			}
			c.ctx.Logger.Info("  Used: %s (%.1f%%)", system.FormatSize(result.UsedSize), percentage)

			if result.Available > 0 {
				c.ctx.Logger.Info("  Available: %s", system.FormatSize(result.Available))
			}
		}
	} else {
		c.ctx.Logger.Info("Status: Not mounted")
	}

	// Display any errors
	if len(result.Errors) > 0 {
		c.ctx.Logger.Info("")
		for _, err := range result.Errors {
			c.ctx.Logger.Warning("%s", err)
		}
		c.ctx.Logger.Info("")
		return fmt.Errorf("info gathering completed with warnings")
	}

	c.ctx.Logger.Info("")
	return nil
}

func (c *InfoCommand) outputError(containerPath string, err error) error {
	if c.jsonOutput {
		result := &InfoResult{
			ContainerPath: containerPath,
			Errors:        []string{err.Error()},
		}
		return c.outputJSON(result)
	}
	return err
}
