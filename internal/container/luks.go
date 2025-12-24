package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	cmd.Stdin = strings.NewReader(string(a.Password.Bytes()) + "\n")
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

// InteractiveAuth uses interactive terminal authentication
type InteractiveAuth struct{}

// Apply applies interactive authentication to a command
func (a *InteractiveAuth) Apply(cmd *exec.Cmd) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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

	// Run the command
	var stdout, stderr strings.Builder
	if _, ok := auth.(*InteractiveAuth); ok {
		// Interactive auth already set stdin/stdout/stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to format LUKS container: %w", err)
		}
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to format LUKS container: %w\nStderr: %s", err, stderr.String())
		}
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

	// Run the command
	var stdout, stderr strings.Builder
	if _, ok := auth.(*InteractiveAuth); ok {
		// Interactive auth already set stdin/stdout/stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to open LUKS container: %w", err)
		}
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to open LUKS container: %w\nStderr: %s", err, stderr.String())
		}
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
