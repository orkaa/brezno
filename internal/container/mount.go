package container

import (
	"fmt"
	"os"
	"strings"

	"github.com/nace/brezno/internal/system"
)

// MountManager handles filesystem mount operations
type MountManager struct {
	executor *system.Executor
}

// NewMountManager creates a new mount manager
func NewMountManager(executor *system.Executor) *MountManager {
	return &MountManager{
		executor: executor,
	}
}

// Mount mounts a device to a mount point
func (m *MountManager) Mount(device, mountPoint string, readonly bool) error {
	// Ensure mount point exists
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	args := []string{}
	if readonly {
		args = append(args, "-o", "ro")
	}
	args = append(args, device, mountPoint)

	err := m.executor.Run("mount", args...)
	if err != nil {
		return fmt.Errorf("failed to mount %s to %s: %w", device, mountPoint, err)
	}

	return nil
}

// Unmount unmounts a mount point
func (m *MountManager) Unmount(mountPoint string, force bool) error {
	if !force {
		return m.executor.Run("umount", mountPoint)
	}

	// Try normal unmount first
	if err := m.executor.Run("umount", mountPoint); err == nil {
		return nil
	}

	// Try force unmount
	if err := m.executor.Run("umount", "-f", mountPoint); err == nil {
		return nil
	}

	// Try lazy unmount as last resort
	return m.executor.Run("umount", "-l", mountPoint)
}

// MakeFilesystem creates a filesystem on a device
func (m *MountManager) MakeFilesystem(device, fsType string) error {
	switch fsType {
	case "ext4":
		return m.executor.Run("mkfs.ext4", "-q", "-L", "encrypted", device)
	case "xfs":
		return m.executor.Run("mkfs.xfs", "-L", "encrypted", device)
	case "btrfs":
		return m.executor.Run("mkfs.btrfs", "-L", "encrypted", device)
	default:
		return fmt.Errorf("unsupported filesystem: %s", fsType)
	}
}

// ResizeFilesystem expands a mounted filesystem to use all available space
func (m *MountManager) ResizeFilesystem(mapperDevice, fsType, mountPoint string) error {
	switch fsType {
	case "ext4":
		// ext4 can resize online, uses the device
		err := m.executor.Run("resize2fs", mapperDevice)
		if err != nil {
			return fmt.Errorf("failed to resize ext4 filesystem: %w", err)
		}

	case "xfs":
		// xfs requires the mount point for online resize
		err := m.executor.Run("xfs_growfs", mountPoint)
		if err != nil {
			return fmt.Errorf("failed to resize xfs filesystem: %w", err)
		}

	case "btrfs":
		// btrfs uses mount point, 'max' means use all available space
		err := m.executor.Run("btrfs", "filesystem", "resize", "max", mountPoint)
		if err != nil {
			return fmt.Errorf("failed to resize btrfs filesystem: %w", err)
		}

	default:
		return fmt.Errorf("unsupported filesystem for resize: %s", fsType)
	}

	return nil
}

// GetFilesystemSize gets the size and usage of a mounted filesystem
func (m *MountManager) GetFilesystemSize(mountPoint string) (size uint64, used uint64, err error) {
	output, err := m.executor.RunOutput("df", "--block-size=1", mountPoint)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get filesystem size: %w", err)
	}

	// Parse df output
	// Header: Filesystem     1B-blocks      Used Available Use% Mounted on
	// Data:   /dev/mapper/x  1234567890  123456  ...
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return 0, 0, fmt.Errorf("invalid df output")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 3 {
		return 0, 0, fmt.Errorf("invalid df output format")
	}

	// Parse size and used
	fmt.Sscanf(fields[1], "%d", &size)
	fmt.Sscanf(fields[2], "%d", &used)

	return size, used, nil
}
