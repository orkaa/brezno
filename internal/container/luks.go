package container

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/nace/brezno/internal/system"
)

// AuthMethod represents a method to authenticate to a LUKS container
type AuthMethod interface {
	Apply(cmd *exec.Cmd) error
}

// PasswordAuth authenticates using a passphrase
type PasswordAuth struct {
	Password *system.SecureBytes
}

// Apply applies password authentication to a command
func (a *PasswordAuth) Apply(cmd *exec.Cmd) error {
	if a.Password == nil {
		return fmt.Errorf("password is nil")
	}
	// Use bytes.NewBuffer to avoid string conversion that would leave password in memory
	cmd.Stdin = bytes.NewBuffer(append(a.Password.Bytes(), '\n'))
	return nil
}

// KeyfileAuth authenticates using a keyfile
type KeyfileAuth struct {
	KeyfilePath string
}

// Apply applies keyfile authentication to a command
func (a *KeyfileAuth) Apply(cmd *exec.Cmd) error {
	cmd.Args = append(cmd.Args, "--key-file", a.KeyfilePath)
	return nil
}

// LUKSManager handles LUKS operations
type LUKSManager struct {
	executor *system.Executor
}

// NewLUKSManager creates a new LUKS manager
func NewLUKSManager(executor *system.Executor) *LUKSManager {
	return &LUKSManager{
		executor: executor,
	}
}

// Format formats a device as LUKS2
func (m *LUKSManager) Format(path string, auth AuthMethod) error {
	cmd := exec.Command("cryptsetup", "luksFormat", "--type", "luks2", path)
	if err := auth.Apply(cmd); err != nil {
		return err
	}

	// Run the command through executor for debug output and sanitization
	_, err := m.executor.RunCmd(cmd)
	if err != nil {
		return fmt.Errorf("failed to format LUKS container: %w", err)
	}

	return nil
}

// IsLUKS checks if a file is LUKS formatted
func (m *LUKSManager) IsLUKS(path string) (bool, error) {
	err := m.executor.Run("cryptsetup", "isLuks", path)
	return err == nil, nil
}

// Open opens a LUKS container
func (m *LUKSManager) Open(device, mapperName string, auth AuthMethod) error {
	cmd := exec.Command("cryptsetup", "luksOpen", device, mapperName)
	if err := auth.Apply(cmd); err != nil {
		return err
	}

	// Run the command through executor for debug output and sanitization
	_, err := m.executor.RunCmd(cmd)
	if err != nil {
		return fmt.Errorf("failed to open LUKS container: %w", err)
	}

	return nil
}

// Close closes a LUKS container
func (m *LUKSManager) Close(mapperName string) error {
	err := m.executor.Run("cryptsetup", "luksClose", mapperName)
	if err != nil {
		return fmt.Errorf("failed to close LUKS container %s: %w", mapperName, err)
	}
	return nil
}

// Resize expands a LUKS container to use all available space on its device
// The mapper must already be open. This requires authentication.
func (m *LUKSManager) Resize(mapperName string, auth AuthMethod) error {
	cmd := exec.Command("cryptsetup", "resize", mapperName)
	if err := auth.Apply(cmd); err != nil {
		return err
	}

	_, err := m.executor.RunCmd(cmd)
	if err != nil {
		return fmt.Errorf("failed to resize LUKS container: %w", err)
	}

	return nil
}

// GetLUKSSize gets the current size of a LUKS container in bytes
func (m *LUKSManager) GetLUKSSize(mapperName string) (uint64, error) {
	mapperDevice := "/dev/mapper/" + mapperName
	output, err := m.executor.RunOutput("blockdev", "--getsize64", mapperDevice)
	if err != nil {
		return 0, fmt.Errorf("failed to get LUKS size: %w", err)
	}

	var size uint64
	_, err = fmt.Sscanf(fmt.Sprintf("%s", output), "%d", &size)
	if err != nil {
		return 0, fmt.Errorf("failed to parse LUKS size: %w", err)
	}

	return size, nil
}

// applyNewAuth applies new authentication method to a command.
// This is different from AuthMethod.Apply() because cryptsetup luksChangeKey
// uses a positional argument for the new keyfile, not a flag.
func applyNewAuth(cmd *exec.Cmd, auth AuthMethod) error {
	switch a := auth.(type) {
	case *KeyfileAuth:
		// Add new keyfile as positional argument
		// cryptsetup luksChangeKey <device> [<new key file>]
		cmd.Args = append(cmd.Args, a.KeyfilePath)
		return nil

	case *PasswordAuth:
		if a.Password == nil {
			return fmt.Errorf("password is nil")
		}

		// For new password via stdin, we need to handle stdin carefully.
		// cryptsetup luksChangeKey reads:
		//   1. Current passphrase from stdin (if no --key-file)
		//   2. New passphrase from stdin (if no new keyfile argument)

		// Check if current auth already set stdin (password→password case)
		if cmd.Stdin != nil {
			// Current auth already set stdin with old password
			// We need to append new password to the existing stdin buffer
			existingStdin, ok := cmd.Stdin.(*bytes.Buffer)
			if !ok {
				return fmt.Errorf("unexpected stdin type: %T", cmd.Stdin)
			}
			existingStdin.Write(a.Password.Bytes())
			existingStdin.WriteByte('\n')
		} else {
			// Current auth is keyfile, only new password goes to stdin
			cmd.Stdin = bytes.NewBuffer(append(a.Password.Bytes(), '\n'))
		}
		return nil

	default:
		return fmt.Errorf("unsupported authentication type: %T", auth)
	}
}

// ChangeKey changes the authentication credentials for LUKS key slot 0.
// Supports all authentication transitions:
//   - password → password
//   - password → keyfile
//   - keyfile → password
//   - keyfile → keyfile
func (m *LUKSManager) ChangeKey(device string, currentAuth, newAuth AuthMethod) error {
	// Build command: cryptsetup luksChangeKey --key-slot 0 <device>
	cmd := exec.Command("cryptsetup", "luksChangeKey", "--key-slot", "0", device)

	// Apply current authentication
	if err := currentAuth.Apply(cmd); err != nil {
		return fmt.Errorf("failed to apply current authentication: %w", err)
	}

	// Apply new authentication
	if err := applyNewAuth(cmd, newAuth); err != nil {
		return fmt.Errorf("failed to apply new authentication: %w", err)
	}

	// Execute through executor for debug output and sanitization
	_, err := m.executor.RunCmd(cmd)
	if err != nil {
		return fmt.Errorf("cryptsetup luksChangeKey failed: %w", err)
	}

	return nil
}
