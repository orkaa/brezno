package container

// Container represents a dm-crypt LUKS container
type Container struct {
	Path       string // Absolute path to container file
	MapperName string // Device mapper name (e.g., crypt_container_img)
	MountPoint string // Where filesystem is mounted
	LoopDevice string // Loop device (e.g., /dev/loop0)
	Filesystem string // ext4, xfs, btrfs
	Size       uint64 // Size in bytes
	Used       uint64 // Used space in bytes
	IsActive   bool   // Currently opened/mounted
}
