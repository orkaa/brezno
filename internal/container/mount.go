package container

import (
	"fmt"
	"os"

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
