package container

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nace/brezno/internal/system"
)

// LoopManager handles loop device operations
type LoopManager struct {
	executor *system.Executor
}

// NewLoopManager creates a new loop manager
func NewLoopManager(executor *system.Executor) *LoopManager {
	return &LoopManager{
		executor: executor,
	}
}

// Attach attaches a file to a loop device
func (m *LoopManager) Attach(path string) (string, error) {
	output, err := m.executor.RunOutput("losetup", "-f", "--show", path)
	if err != nil {
		return "", fmt.Errorf("failed to attach loop device: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// Detach detaches a loop device
func (m *LoopManager) Detach(device string) error {
	err := m.executor.Run("losetup", "-d", device)
	if err != nil {
		return fmt.Errorf("failed to detach loop device %s: %w", device, err)
	}
	return nil
}

// FindByFile finds the loop device for a file
func (m *LoopManager) FindByFile(path string) (string, error) {
	output, err := m.executor.RunOutput("losetup", "-j", path)
	if err != nil || output == "" {
		return "", nil
	}

	// Parse: "/dev/loop0: []: (/path/to/file)"
	parts := strings.SplitN(output, ":", 2)
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0]), nil
	}
	return "", nil
}

// losetupDevice represents a loop device from losetup -l -J output
type losetupDevice struct {
	Name     string `json:"name"`
	BackFile string `json:"back-file"`
}

type losetupOutput struct {
	LoopDevices []losetupDevice `json:"loopdevices"`
}

// GetAll returns all loop devices with their backing files
func (m *LoopManager) GetAll() (map[string]string, error) {
	output, err := m.executor.RunOutput("losetup", "-l", "-J")
	if err != nil {
		return nil, fmt.Errorf("failed to list loop devices: %w", err)
	}

	var result losetupOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("failed to parse losetup output: %w", err)
	}

	devices := make(map[string]string)
	for _, dev := range result.LoopDevices {
		if dev.BackFile != "" {
			devices[dev.Name] = dev.BackFile
		}
	}

	return devices, nil
}
