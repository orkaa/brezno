package container

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nace/brezno/internal/system"
)

// Discovery handles container discovery by querying system state
type Discovery struct {
	executor    *system.Executor
	loopManager *LoopManager
}

// NewDiscovery creates a new discovery instance
func NewDiscovery(executor *system.Executor) *Discovery {
	return &Discovery{
		executor:    executor,
		loopManager: NewLoopManager(executor),
	}
}

// DiscoverActive discovers all active LUKS containers
func (d *Discovery) DiscoverActive() ([]Container, error) {
	// Step 1: Get all crypt-type mapper devices
	mappers, err := d.getCryptMappers()
	if err != nil {
		return nil, err
	}

	// Step 2: Get all loop devices and their backing files
	loopDevices, err := d.loopManager.GetAll()
	if err != nil {
		return nil, err
	}

	// Step 3: Parse /proc/mounts to find mount points
	mounts, err := d.getMounts()
	if err != nil {
		return nil, err
	}

	// Step 4: Correlate all information
	var containers []Container
	for _, mapper := range mappers {
		container := Container{
			MapperName: mapper,
			IsActive:   true,
		}

		// Get backing loop device from dmsetup table
		loopDev, err := d.getMapperLoopDevice(mapper)
		if err != nil {
			continue
		}
		container.LoopDevice = loopDev

		// Get container file from loop device
		if backFile, ok := loopDevices[loopDev]; ok {
			absPath, _ := filepath.Abs(backFile)
			container.Path = absPath
		}

		// Get mount information
		mapperDevice := "/dev/mapper/" + mapper
		if mount, ok := mounts[mapperDevice]; ok {
			container.MountPoint = mount.MountPoint
			container.Filesystem = mount.Filesystem
			container.Size = mount.Size
			container.Used = mount.Used
		}

		containers = append(containers, container)
	}

	return containers, nil
}

// FindByPath finds a container by its file path
func (d *Discovery) FindByPath(path string) (*Container, error) {
	containers, err := d.DiscoverActive()
	if err != nil {
		return nil, err
	}

	absPath, _ := filepath.Abs(path)
	for _, c := range containers {
		if c.Path == absPath {
			return &c, nil
		}
	}

	return nil, nil
}

// FindByMapper finds a container by its mapper name
func (d *Discovery) FindByMapper(mapper string) (*Container, error) {
	containers, err := d.DiscoverActive()
	if err != nil {
		return nil, err
	}

	for _, c := range containers {
		if c.MapperName == mapper {
			return &c, nil
		}
	}

	return nil, nil
}

// FindByMount finds a container by its mount point
func (d *Discovery) FindByMount(mount string) (*Container, error) {
	containers, err := d.DiscoverActive()
	if err != nil {
		return nil, err
	}

	absMount, _ := filepath.Abs(mount)
	for _, c := range containers {
		if c.MountPoint == absMount {
			return &c, nil
		}
	}

	return nil, nil
}

// getCryptMappers returns all crypt-type device mapper names
func (d *Discovery) getCryptMappers() ([]string, error) {
	output, err := d.executor.RunOutput("dmsetup", "ls", "--target", "crypt")
	if err != nil {
		// dmsetup returns error if no devices found
		return []string{}, nil
	}

	var mappers []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		// Format: "mapper_name    (major, minor)"
		parts := strings.Fields(line)
		if len(parts) > 0 {
			mappers = append(mappers, parts[0])
		}
	}

	return mappers, nil
}

// getMapperLoopDevice gets the backing loop device for a mapper
func (d *Discovery) getMapperLoopDevice(mapper string) (string, error) {
	output, err := d.executor.RunOutput("dmsetup", "table", mapper)
	if err != nil {
		return "", err
	}

	// Parse dmsetup table output
	// Format: "0 sectors crypt cipher ... backing_device offset"
	device, err := system.ParseDmsetupTable(output)
	if err != nil {
		return "", err
	}

	// Convert major:minor format (e.g., "7:2") to device path (e.g., "/dev/loop2")
	// Loop devices always have major number 7
	if strings.Contains(device, ":") {
		parts := strings.Split(device, ":")
		if len(parts) == 2 && parts[0] == "7" {
			device = "/dev/loop" + parts[1]
		}
	}

	return device, nil
}

// MountInfo represents mount information
type MountInfo struct {
	Device     string
	MountPoint string
	Filesystem string
	Size       uint64
	Used       uint64
}

// getMounts parses /proc/mounts to find mount points
func (d *Discovery) getMounts() (map[string]MountInfo, error) {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil, err
	}

	mounts := make(map[string]MountInfo)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 {
			device := fields[0]
			// Only track /dev/mapper/* devices
			if strings.HasPrefix(device, "/dev/mapper/") {
				info := MountInfo{
					Device:     device,
					MountPoint: fields[1],
					Filesystem: fields[2],
				}

				// Try to get size information using df
				if size, used, err := d.getDiskUsage(fields[1]); err == nil {
					info.Size = size
					info.Used = used
				}

				mounts[device] = info
			}
		}
	}

	return mounts, nil
}

// getDiskUsage gets disk usage for a mount point
func (d *Discovery) getDiskUsage(mountPoint string) (size uint64, used uint64, err error) {
	output, err := d.executor.RunOutput("df", "--block-size=1", mountPoint)
	if err != nil {
		return 0, 0, err
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
